package ovn

import (
	"fmt"
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
	DriverName          = "ovn"
	localhost           = "127.0.0.1"
	bridgePrefix        = "ovnbr-"
	bridgeNameOption    = "net.libnetwork.ovn.bridge.name"
	bindInterfaceOption = "net.libnetwork.ovn.bridge.bind_interface"

	mtuOption  = "net.libnetwork.ovn.bridge.mtu"
	modeOption = "net.libnetwork.ovn.bridge.mode"

	modeNAT  = "nat"
	modeFlat = "flat"

	defaultMTU  = 1500
	defaultMode = modeNAT

	ovnNBPort = 6641
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
	dockerer
	networks map[string]*NetworkState
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

type ovnnber struct {
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

// NewDriver creates an OVN driver
func NewDriver() (*Driver, error) {
	docker, err := dockerclient.NewDockerClient("unix:///var/run/docker.sock", nil)
	if err != nil {
		return nil, fmt.Errorf("could not connect to docker: %s", err)
	}

	// initiate the ovsdb manager port binding
	var ovnnb *libovsdb.OvsdbClient
	retries := 3
	for i := 0; i < retries; i++ {
		ovnnb, err = libovsdb.Connect(localhost, ovnNBPort)
		if err == nil {
			break
		}
		log.Errorf("could not connect to OVN Northbound on port [ %d ]: %s. Retrying in 5 seconds", ovnNBPort, err)
		time.Sleep(5 * time.Second)
	}

	if ovnnb == nil {
		return nil, fmt.Errorf("could not connect to OVN Northbound")
	}

	d := &Driver{
		dockerer: dockerer{
			client: docker,
		},
		ovnnber: ovnnber{
			ovsdb: ovnnb,
		},
		networks: make(map[string]*NetworkState),
	}
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
	log.Debugf("Create endpoint request: %+v", req)
	fmt.Println("Create endpoint request", req)
	return nil, nil
}

// DeleteEndpoint deletes a logical switch port
func (d *Driver) DeleteEndpoint(req *network.DeleteEndpointRequest) error {
	return nil
}

// CreateNetwork creates a logical switch
func (d *Driver) CreateNetwork(req *network.CreateNetworkRequest) error {
	fmt.Printf("Create network request: %+v\n", req)

	bridgeName, err := getBridgeName(req)
	if err != nil {
		return err
	}
	fmt.Println("Bridge name:", bridgeName)

	mtu, err := getBridgeMTU(req)
	if err != nil {
		return err
	}
	fmt.Println("MTU:", mtu)

	mode, err := getBridgeMode(req)
	if err != nil {
		return err
	}
	fmt.Println("Mode:", mode)

	gateway, mask, err := getGatewayIP(req)
	if err != nil {
		return err
	}
	fmt.Println("Gateway mask:", gateway, mask)

	bindInterface, err := getBindInterface(req)
	if err != nil {
		return err
	}
	fmt.Println("Bindinterface:", bindInterface)

	ns := &NetworkState{
		BridgeName:        bridgeName,
		MTU:               mtu,
		Mode:              mode,
		Gateway:           gateway,
		GatewayMask:       mask,
		FlatBindInterface: bindInterface,
	}

	d.networks[req.NetworkID] = ns

	log.Infof("Initializing bridge for network %s", req.NetworkID)
	if err := d.initBridge(req.NetworkID); err != nil {
		delete(d.networks, req.NetworkID)
		return err
	}
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
	return nil, nil
}

// FreeNetwork frees a logical switch
func (d *Driver) FreeNetwork(req *network.FreeNetworkRequest) error {
	return nil
}

// GetCapabilities returns scope
func (d *Driver) GetCapabilities() (*network.CapabilitiesResponse, error) {
	res := &network.CapabilitiesResponse{
		Scope: network.LocalScope,
	}
	return res, nil
}

// Join is invoked when a Sandbox is attached to an endpoint.
func (d *Driver) Join(req *network.JoinRequest) (*network.JoinResponse, error) {
	return nil, nil
}

// Leave method is invoked when a Sandbox detaches from an endpoint.
func (d *Driver) Leave(req *network.LeaveRequest) error {
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
