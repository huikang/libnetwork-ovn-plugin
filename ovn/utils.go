package ovn

import (
	"fmt"
	"net"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/ns"
	"github.com/vishvananda/netlink"
)

// Generate a mac addr
func makeMac(ip net.IP) string {
	hw := make(net.HardwareAddr, 6)
	hw[0] = 0x7a
	hw[1] = 0x42
	copy(hw[2:], ip.To4())
	return hw.String()
}

// Return the IPv4 address of a network interface
func getIfaceAddr(name string) (*net.IPNet, error) {
	iface, err := netlink.LinkByName(name)
	if err != nil {
		return nil, err
	}
	addrs, err := netlink.AddrList(iface, netlink.FAMILY_V4)
	if err != nil {
		return nil, err
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("Interface %s has no IP addresses", name)
	}
	if len(addrs) > 1 {
		log.Infof("Interface [ %v ] has more than 1 IPv4 address. Defaulting to using [ %v ]\n", name, addrs[0].IP)
	}
	return addrs[0].IPNet, nil
}

// Set the IP addr of a netlink interface
func setInterfaceIP(name string, rawIP string) error {
	retries := 2
	var iface netlink.Link
	var err error
	for i := 0; i < retries; i++ {
		iface, err = netlink.LinkByName(name)
		if err == nil {
			break
		}
		log.Debugf("error retrieving new OVS bridge netlink link [ %s ]... retrying", name)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("Abandoning retrieving the new OVS bridge link from netlink, Run [ ip link ] to troubleshoot the error: %s", err)
		return err
	}
	ipNet, err := netlink.ParseIPNet(rawIP)
	if err != nil {
		return err
	}

	// fixmehk: The last argument could be wrong.
	addr := &netlink.Addr{
		IPNet:     ipNet,
		Label:     "",
		Flags:     0,
		Scope:     0,
		Peer:      nil,
		Broadcast: net.IP("127.0.0.1"),
	}
	return netlink.AddrAdd(iface, addr)
}

// Increment an IP in a subnet
func ipIncrement(networkAddr net.IP) net.IP {
	for i := 15; i >= 0; i-- {
		b := networkAddr[i]
		if b < 255 {
			networkAddr[i] = b + 1
			for xi := i + 1; xi <= 15; xi++ {
				networkAddr[xi] = 0
			}
			break
		}
	}
	return networkAddr
}

// Check if a netlink interface exists in the default namespace
func validateIface(ifaceStr string) bool {
	_, err := net.InterfaceByName(ifaceStr)
	if err != nil {
		log.Debugf("The requested interface [ %s ] was not found on the host: %s", ifaceStr, err)
		return false
	}
	return true
}

func createVethPair(vethOut, vethIn, mac string) error {
	log.Infof("Create veth [%s %s]", vethOut, vethIn)

	nlh := ns.NlHandle()

	// Generate and add the interface pipe host <-> sandbox
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: vethOut, TxQLen: 0},
		PeerName:  vethIn}
	fmt.Println("type:", veth.Type())

	if err := nlh.LinkAdd(veth); err != nil {
		return fmt.Errorf("error creating veth pair: %v", err)
	}

	// command = "ip link set dev %s address %s" % (veth_inside, mac_address)
	l, err := nlh.LinkByName(vethIn)
	if err != nil {
		return fmt.Errorf("failed to get link by name %s : %s", vethIn, err.Error())
	}

	hwAddr, err := net.ParseMAC(mac)
	if err != nil {
		return fmt.Errorf("failed to parse mac %s : %s", mac, err.Error())
	}

	if err := nlh.LinkSetHardwareAddr(l, hwAddr); err != nil {
		return fmt.Errorf("failed to set bridge mac-address %s : %s", hwAddr, err.Error())
	}

	// command = "ip link set %s up" % (veth_outside)
	l, err = nlh.LinkByName(vethOut)
	if err != nil {
		return fmt.Errorf("failed to get link by name %s : %s", vethIn, err.Error())
	}
	if err := nlh.LinkSetUp(l); err != nil {
		return fmt.Errorf("failed to set link up %s", err.Error())
	}
	return nil
}
