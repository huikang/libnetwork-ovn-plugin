package ovn

import (
	"fmt"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/go-plugins-helpers/network"
	"github.com/samalba/dockerclient"
	"github.com/socketplane/libovsdb"
)

const (
	DriverName = "ovn"
	localhost  = "127.0.0.1"

	ovnNBPort = 6641
)

type dockerer struct {
	client *dockerclient.DockerClient
}

type Driver struct {
	ovnnber
	dockerer
}

type ovnnber struct {
	ovsdb *libovsdb.OvsdbClient
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
	}
	return d, nil
}

func (d *Driver) AllocateNetwork(req *network.AllocateNetworkRequest) (*network.AllocateNetworkResponse, error) {
	log.Debugf("Allocate network request: %+v", req)
	res := &network.AllocateNetworkResponse{
		Options: make(map[string]string),
	}
	return res, nil
}

func (d *Driver) CreateEndpoint(req *network.CreateEndpointRequest) (*network.CreateEndpointResponse, error) {
	log.Debugf("Create network request: %+v", req)
	fmt.Println("Create network request", req)
	return nil, nil
}

func (d *Driver) DeleteEndpoint(req *network.DeleteEndpointRequest) error {
	return nil
}

func (d *Driver) CreateNetwork(req *network.CreateNetworkRequest) error {
	gateway, mask, err := getGatewayIP(req)
	if err != nil {
		return err
	}

	fmt.Println("Gateway", gateway, mask)

	return nil
}

func (d *Driver) DeleteNetwork(req *network.DeleteNetworkRequest) error {
	return nil
}

func (d *Driver) DiscoverDelete(req *network.DiscoveryNotification) error {
	return nil
}

func (d *Driver) DiscoverNew(notif *network.DiscoveryNotification) error {
	return nil
}

func (d *Driver) EndpointInfo(req *network.InfoRequest) (*network.InfoResponse, error) {
	return nil, nil
}

func (d *Driver) FreeNetwork(req *network.FreeNetworkRequest) error {
	return nil
}

func (d *Driver) GetCapabilities() (*network.CapabilitiesResponse, error) {
	res := &network.CapabilitiesResponse{
		Scope: network.LocalScope,
	}
	return res, nil
}

func (d *Driver) Join(req *network.JoinRequest) (*network.JoinResponse, error) {
	return nil, nil
}

func (d *Driver) Leave(req *network.LeaveRequest) error {
	return nil
}

func (d *Driver) ProgramExternalConnectivity(req *network.ProgramExternalConnectivityRequest) error {
	return nil
}

func (d *Driver) RevokeExternalConnectivity(req *network.RevokeExternalConnectivityRequest) error {
	return nil
}
