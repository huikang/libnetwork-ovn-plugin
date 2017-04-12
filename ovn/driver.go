package ovn

import (
	"fmt"
	"net"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/go-plugins-helpers/network"
	"github.com/samalba/dockerclient"
	"github.com/socketplane/libovsdb"
	"github.com/vishvananda/netlink"
)

const (
	// DriverName is the ovn plugin name
	DriverName = "ovn"
	// Localhost is the default ovsdb host
	Localhost = "127.0.0.1"

	bridgePrefix        = "ovnbr-"
	ovnbridge           = "br-int"
	containerEthName    = "eth"
	bridgeNameOption    = "net.libnetwork.ovn.bridge.name"
	bindInterfaceOption = "net.libnetwork.ovn.bridge.bind_interface"

	mtuOption  = "net.libnetwork.ovn.bridge.mtu"
	modeOption = "net.libnetwork.ovn.bridge.mode"

	modeNAT  = "nat"
	modeFlat = "flat"

	defaultMTU  = 1500
	defaultMode = modeNAT
)

var (
	validModes = map[string]bool{
		modeNAT:  true,
		modeFlat: true,
	}
)

type dockerer struct {
	client *dockerclient.DockerClient
}

// Driver is ovn driver strcut
type Driver struct {
	ovnnber
	ovsdber
	dockerer
	networks  map[string]*NetworkState
	endpoints map[string]*EndpointState
}

// NetworkState is filled in at network creation time
// it contains state that we wish to keep for each network
type NetworkState struct {
	BridgeName        string
	MTU               int
	Mode              string
	Gateway           string
	GatewayMask       string
	FlatBindInterface string
}

// EndpointState is filled in at network creation time
// it contains state that we wish to keep for each network
type EndpointState struct {
	LogicalPortName string
	addr            string
	mac             string
	vethOut         string
	vethIn          string
}

type ovnnber struct {
	ovsdb *libovsdb.OvsdbClient
}

type ovsdber struct {
	ovsdb *libovsdb.OvsdbClient
}

// Enable a netlink interface
func interfaceUp(name string) error {
	iface, err := netlink.LinkByName(name)
	if err != nil {
		log.Debugf("Error retrieving a link named [ %s ]", iface.Attrs().Name)
		return err
	}
	return netlink.LinkSetUp(iface)
}

func truncateID(id string) string {
	return id[:5]
}

func getBridgeName(r *network.CreateNetworkRequest) (string, error) {
	bridgeName := bridgePrefix + truncateID(r.NetworkID)
	if r.Options != nil {
		if name, ok := r.Options[bridgeNameOption].(string); ok {
			bridgeName = name
		}
	}
	return bridgeName, nil
}

func getLogicalPortName(req *network.CreateEndpointRequest) string {
	logicalPortName := "br" + truncateID(req.NetworkID) + "-" + truncateID(req.EndpointID)
	return logicalPortName
}

func getBridgeMTU(r *network.CreateNetworkRequest) (int, error) {
	bridgeMTU := defaultMTU
	if r.Options != nil {
		if mtu, ok := r.Options[mtuOption].(int); ok {
			bridgeMTU = mtu
		}
	}
	return bridgeMTU, nil
}

func getBridgeMode(r *network.CreateNetworkRequest) (string, error) {
	bridgeMode := defaultMode
	if r.Options != nil {
		if mode, ok := r.Options[modeOption].(string); ok {
			if _, isValid := validModes[mode]; !isValid {
				return "", fmt.Errorf("%s is not a valid mode", mode)
			}
			bridgeMode = mode
		}
	}
	return bridgeMode, nil
}

func getGatewayIP(r *network.CreateNetworkRequest) (string, string, error) {
	// FIXME: Dear future self, I'm sorry for leaving you with this mess, but I want to get this working ASAP
	// This should be an array
	// We need to handle case where we have
	// a. v6 and v4 - dual stack
	// auxilliary address
	// multiple subnets on one network
	// also in that case, we'll need a function to determine the correct default gateway based on it's IP/Mask
	var gatewayIP string

	if len(r.IPv6Data) > 0 {
		if r.IPv6Data[0] != nil {
			if r.IPv6Data[0].Gateway != "" {
				gatewayIP = r.IPv6Data[0].Gateway
			}
		}
	}
	// Assumption: IPAM will provide either IPv4 OR IPv6 but not both
	// We may want to modify this in future to support dual stack
	if len(r.IPv4Data) > 0 {
		if r.IPv4Data[0] != nil {
			if r.IPv4Data[0].Gateway != "" {
				gatewayIP = r.IPv4Data[0].Gateway
			}
		}
	}

	if gatewayIP == "" {
		return "", "", fmt.Errorf("No gateway IP found")
	}
	parts := strings.Split(gatewayIP, "/")
	if parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("Cannot split gateway IP address")
	}
	return parts[0], parts[1], nil
}

func getBindInterface(r *network.CreateNetworkRequest) (string, error) {
	if r.Options != nil {
		if mode, ok := r.Options[bindInterfaceOption].(string); ok {
			return mode, nil
		}
	}
	// As bind interface is optional and has no default, don't return an error
	return "", nil
}

func getLogicalPortNamefromresource(nid, eid string) string {
	logicalPortName := "br" + truncateID(nid) + "-" + truncateID(eid)
	return logicalPortName
}

func getBridgeNamefromresource(r *dockerclient.NetworkResource) (string, error) {
	bridgeName := bridgePrefix + truncateID(r.ID)
	if r.Options != nil {
		if name, ok := r.Options[bridgeNameOption]; ok {
			bridgeName = name
		}
	}
	return bridgeName, nil
}

func getInterfaceInfo(req *network.CreateEndpointRequest) (ipaddr, mac string, err error) {
	iface := req.Interface
	if iface == nil {
		return "", "", fmt.Errorf("request does not provide interface")
	}

	if iface.Address == "" {
		return "", "", fmt.Errorf("interface does not provide address")
	}

	cidr, _, err := net.ParseCIDR(iface.Address)
	if err != nil {
		return "", "", fmt.Errorf("invalid IPv4 CIDR [ %s ]", iface.Address)
	}

	if iface.MacAddress == "" {
		mac = makeMac(cidr)
	} else {
		mac = iface.MacAddress
	}

	ipaddr = cidr.String()

	return ipaddr, mac, nil
}

// NewDriver creates an OVN driver
func NewDriver(nbip string) (*Driver, error) {
	docker, err := dockerclient.NewDockerClient("unix:///var/run/docker.sock", nil)
	if err != nil {
		return nil, fmt.Errorf("could not connect to docker: %s", err)
	}

	// initiate the ovn-nb manager port binding
	var ovnnb *libovsdb.OvsdbClient
	retries := 3
	for i := 0; i < retries; i++ {
		ovnnb, err = libovsdb.Connect(nbip, ovnNBPort)
		if err == nil {
			break
		}
		log.Errorf("could not connect to OVN Northbound on port [ %d ]: %s. Retrying in 5 seconds", ovnNBPort, err)
		time.Sleep(5 * time.Second)
	}

	if ovnnb == nil {
		return nil, fmt.Errorf("could not connect to OVN Northbound")
	}

	// initiate the ovsdb manager port binding
	var ovsdb *libovsdb.OvsdbClient
	retries = 3
	for i := 0; i < retries; i++ {
		ovsdb, err = libovsdb.Connect(Localhost, ovsdbPort)
		if err == nil {
			break
		}
		log.Errorf("could not connect to OVSDB on port [ %d ]: %s. Retrying in 5 seconds", ovsdbPort, err)
		time.Sleep(5 * time.Second)
	}

	if ovsdb == nil {
		return nil, fmt.Errorf("could not connect to OVSDB")
	}

	d := &Driver{
		dockerer: dockerer{
			client: docker,
		},
		ovnnber: ovnnber{
			ovsdb: ovnnb,
		},
		ovsdber: ovsdber{
			ovsdb: ovsdb,
		},
		networks:  make(map[string]*NetworkState),
		endpoints: make(map[string]*EndpointState),
	}
	//recover networks and endpoints
	netlist, err := d.dockerer.client.ListNetworks("")
	if err != nil {
		return nil, fmt.Errorf("could not get  docker networks: %s", err)
	}
	for _, net := range netlist {
		if net.Driver == DriverName {
			netInspect, err := d.dockerer.client.InspectNetwork(net.ID)
			if err != nil {
				return nil, fmt.Errorf("could not inpect docker networks inpect: %s", err)
			}
			bridgeName, err := getBridgeNamefromresource(netInspect)
			if err != nil {
				return nil, err
			}
			ns := &NetworkState{
				BridgeName: bridgeName,
			}
			d.networks[net.ID] = ns
			log.Debugf("exist network create by this driver:%v", netInspect.Name)

			for c, ep := range netInspect.Containers {
				log.Debugf("Container name: %v eid %v", c, ep)
				logicalPortName := getLogicalPortNamefromresource(net.ID, ep.EndpointID)
				es := &EndpointState{
					LogicalPortName: logicalPortName,
					addr:            ep.IPv4Address,
					mac:             ep.MacAddress,
					vethOut:         ep.EndpointID[0:15],
				}
				d.endpoints[ep.EndpointID] = es
				log.Debugf("exist endpoint: %v", d.endpoints[ep.EndpointID])
			}
		}
	}

	// fixmehk: add the following setup
	// ovs_vsctl("set", "open_vswitch", ".",
	//	"external_ids:ovn-bridge=" + OVN_BRIDGE); OVN_BRIDGE=br-int
	return d, nil
}

// AllocateNetwork allows a network
func (d *Driver) AllocateNetwork(req *network.AllocateNetworkRequest) (*network.AllocateNetworkResponse, error) {
	log.Debugf("Allocate network request: %+v", req)
	res := &network.AllocateNetworkResponse{
		Options: make(map[string]string),
	}
	return res, nil
}

// CreateEndpoint creates an logical switch port
func (d *Driver) CreateEndpoint(req *network.CreateEndpointRequest) (*network.CreateEndpointResponse, error) {
	log.Infof("Create endpoint request: %+v", req)

	if _, ok := d.networks[req.NetworkID]; !ok {
		return nil, fmt.Errorf("failed to find logical switch for network id [ %s ]", req.NetworkID)
	}
	bridgeName := d.networks[req.NetworkID].BridgeName
	log.Debugf("Bridge name: [ %s ]", bridgeName)

	logicalPortName := getLogicalPortName(req)
	log.Debugf("LogicalPort name: [ %s ]", logicalPortName)

	ipaddr, macaddr, err := getInterfaceInfo(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get interface [ %s ]", req.EndpointID)
	}
	log.Debugf("Interface addr [ %s ] mac [ %s ]", ipaddr, macaddr)

	// 1. Create logical port in NB
	// 1.1 ovn_nbctl("lsp-add", nid, eid)
	// 1.2 ovn_nbctl("lsp-set-addresses", eid, mac_address + " " + ip_address)
	es := &EndpointState{
		LogicalPortName: logicalPortName,
		addr:            ipaddr,
		mac:             macaddr,
	}
	d.endpoints[req.EndpointID] = es

	if err := d.createEndpoint(bridgeName, logicalPortName); err != nil {
		delete(d.endpoints, req.EndpointID)
		return nil, fmt.Errorf("ovn failed to create endpoint")
	}

	if err := d.setEndpointAddr(logicalPortName, ipaddr, macaddr); err != nil {
		return nil, fmt.Errorf("ovn failed to set endpoint addr")
	}

	res := &network.CreateEndpointResponse{
		Interface: &network.EndpointInterface{
			MacAddress: macaddr,
		},
	}
	log.Infof("Created logical port [ %s ] for endpoint id [ %v ]", es.LogicalPortName, req.EndpointID)
	return res, nil
}

// DeleteEndpoint deletes a logical switch port
func (d *Driver) DeleteEndpoint(req *network.DeleteEndpointRequest) error {
	log.Infof("Delete endpoint request: %+v", req)

	if _, ok := d.networks[req.NetworkID]; !ok {
		return fmt.Errorf("failed to find logical switch for network id [ %s ]", req.NetworkID)
	}
	bridgeName := d.networks[req.NetworkID].BridgeName
	log.Infof("Bridge name: %s", bridgeName)

	if _, ok := d.endpoints[req.EndpointID]; !ok {
		return fmt.Errorf("failed to find endpoint for id [ %s ]", req.NetworkID)
	}
	endpointName := d.endpoints[req.EndpointID].LogicalPortName
	log.Infof("Endpoint name: %s", endpointName)

	if err := d.deleteEndpoint(bridgeName, endpointName); err != nil {
		return fmt.Errorf("ovn failed to set endpoint addr")
	}

	return nil
}

// CreateNetwork creates a logical switch
func (d *Driver) CreateNetwork(req *network.CreateNetworkRequest) error {
	log.Infof("Create network request: %+v\n", req)

	bridgeName, err := getBridgeName(req)
	if err != nil {
		return err
	}
	log.Debugf("Bridge name: [ %s ]", bridgeName)

	mtu, err := getBridgeMTU(req)
	if err != nil {
		return err
	}
	log.Debugf("MTU: [ %v ]", mtu)

	mode, err := getBridgeMode(req)
	if err != nil {
		return err
	}
	log.Debugf("Mode: [ %v ]", mode)

	gateway, mask, err := getGatewayIP(req)
	if err != nil {
		return err
	}
	log.Debugf("Gateway mask: [ %v/%v ]", gateway, mask)

	bindInterface, err := getBindInterface(req)
	if err != nil {
		return err
	}
	log.Debugf("Bindinterface: [ %v ]", bindInterface)

	ns := &NetworkState{
		BridgeName:        bridgeName,
		MTU:               mtu,
		Mode:              mode,
		Gateway:           gateway,
		GatewayMask:       mask,
		FlatBindInterface: bindInterface,
	}

	d.networks[req.NetworkID] = ns

	log.Debugf("Initializing bridge for network %s", req.NetworkID)
	if err := d.initBridge(req.NetworkID); err != nil {
		delete(d.networks, req.NetworkID)
		return err
	}
	log.Infof("Created logical bridge [ %s ] for network id [ %v ]", ns.BridgeName, req.NetworkID)
	return nil
}

// DeleteNetwork deletes the logical switch
func (d *Driver) DeleteNetwork(req *network.DeleteNetworkRequest) error {
	return nil
}

// DiscoverDelete is a notification for a discovery delete event, such as a node leaving a cluster
func (d *Driver) DiscoverDelete(req *network.DiscoveryNotification) error {
	return nil
}

// DiscoverNew is a notification for a new discovery event, such as a new node joining a cluster
func (d *Driver) DiscoverNew(notif *network.DiscoveryNotification) error {
	return nil
}

// EndpointInfo gets the endpoint info
func (d *Driver) EndpointInfo(req *network.InfoRequest) (*network.InfoResponse, error) {
	log.Infof("Request EndpointInfo %+v", req)
	if _, ok := d.endpoints[req.EndpointID]; !ok {
		return nil, fmt.Errorf("failed to find endpoint for id [ %s ]", req.NetworkID)
	}
	ep := d.endpoints[req.EndpointID]

	log.Infof("Request EndpointInfo [ %s %s %s]", ep.addr, ep.mac, ep.vethOut)

	resMap := map[string]string{
		"ip_address":   ep.addr,
		"mac_address":  ep.mac,
		"veth_outside": ep.vethOut,
	}
	res := &network.InfoResponse{
		Value: resMap,
	}
	return res, nil
}

// FreeNetwork frees a logical switch
func (d *Driver) FreeNetwork(req *network.FreeNetworkRequest) error {
	return nil
}

// GetCapabilities returns scope
func (d *Driver) GetCapabilities() (*network.CapabilitiesResponse, error) {
	res := &network.CapabilitiesResponse{
		Scope: network.GlobalScope,
	}
	return res, nil
}

// Join is invoked when a Sandbox is attached to an endpoint.
func (d *Driver) Join(req *network.JoinRequest) (*network.JoinResponse, error) {
	log.Infof("Join request: %+v", req)

	if _, ok := d.networks[req.NetworkID]; !ok {
		return nil, fmt.Errorf("failed to find logical switch for network id [ %s ]", req.NetworkID)
	}
	bridgeName := d.networks[req.NetworkID].BridgeName
	log.Infof("Bridge name: %s", bridgeName)

	if _, ok := d.endpoints[req.EndpointID]; !ok {
		return nil, fmt.Errorf("failed to find endpoint for id [ %s ]", req.NetworkID)
	}
	ep := d.endpoints[req.EndpointID]
	log.Infof("Endpoint name: %s [%s %s]", ep.LogicalPortName, ep.mac, ep.addr)

	if req.SandboxKey == "" {
		return nil, fmt.Errorf("failed to find get sandbox key in req")
	}
	sboxkey := req.SandboxKey
	log.Infof("Sandbox key: %s", sboxkey)
	s := strings.Split(sboxkey, "/")
	cnid := s[len(s)-1]
	log.Infof("Sandbox cni key: %s", cnid)

	vethOut := req.EndpointID[0:15]
	vethIn := req.EndpointID[0:13] + "_c"
	if err := createVethPair(vethOut, vethIn, ep.mac); err != nil {
		return nil, fmt.Errorf("failed to create veth pair")
	}
	ep.vethOut = vethOut
	ep.vethIn = vethIn
	log.Debugf("Created veth %s:%s", ep.vethOut, ep.vethIn)

	// ovs_vsctl("add-port", OVN_BRIDGE, veth_outside)
	// ovs_vsctl("set", "interface", veth_outside,
	//	"external_ids:attached-mac=" + mac_address,
	//	"external_ids:iface-id=" + eid,
	//	"external_ids:vm-id=" + vm_id,
	//	"external_ids:iface-status=active")
	if err := d.addVethPort(ovnbridge, vethOut, ep.mac, ep.LogicalPortName, cnid); err != nil {
		return nil, fmt.Errorf("ovn failed to join endpoint [ %s ] to sb [ %s ]", vethOut, sboxkey)
	}

	res := &network.JoinResponse{
		InterfaceName: network.InterfaceName{
			SrcName:   ep.vethIn,
			DstPrefix: containerEthName,
		},
		Gateway: d.networks[req.NetworkID].Gateway,
	}
	log.Debugf("Join endpoint %s:%s to %s", req.NetworkID, req.EndpointID, req.SandboxKey)

	return res, nil
}

// Leave method is invoked when a Sandbox detaches from an endpoint.
func (d *Driver) Leave(req *network.LeaveRequest) error {
	log.Infof("Leave request: %+v", req)

	if _, ok := d.networks[req.NetworkID]; !ok {
		return fmt.Errorf("failed to find logical switch for network id [ %s ]", req.NetworkID)
	}
	bridgeName := d.networks[req.NetworkID].BridgeName
	log.Infof("Bridge name: %s", bridgeName)

	if _, ok := d.endpoints[req.EndpointID]; !ok {
		return fmt.Errorf("failed to find endpoint for id [ %s ]", req.NetworkID)
	}
	ep := d.endpoints[req.EndpointID]
	log.Debugf("Endpoint name: %s [%s %s %s]", ep.LogicalPortName, ep.mac, ep.addr, ep.vethOut)

	// command = "ip link delete %s" % (veth_outside)
	iface, err := netlink.LinkByName(ep.vethOut)
	if err != nil {
		return fmt.Errorf("Error retrieving a link named [ %s ]", iface.Attrs().Name)
	}
	if err := netlink.LinkDel(iface); err != nil {
		log.Errorf("unable to delete veth on leave: %s", err)
	}
	log.Infof("Deleted link veth [ %s ]", ep.vethOut)

	// ovs_vsctl("--if-exists", "del-port", veth_outside)
	if err := d.ovsdber.deletePort(ovnbridge, ep.vethOut); err != nil {
		return fmt.Errorf("ovs failed to delete port")
	}
	delete(d.endpoints, req.EndpointID)
	log.Infof("Deleted port [ %s ] on OVN bridge [ %v ]", ep.LogicalPortName, ovnbridge)
	return nil
}

// ProgramExternalConnectivity external
func (d *Driver) ProgramExternalConnectivity(req *network.ProgramExternalConnectivityRequest) error {
	return nil
}

// RevokeExternalConnectivity revokes
func (d *Driver) RevokeExternalConnectivity(req *network.RevokeExternalConnectivityRequest) error {
	return nil
}
