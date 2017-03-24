package ovn

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/go-plugins-helpers/network"
	"github.com/samalba/dockerclient"
)

const (
	DriverName = "ovn"
)

type dockerer struct {
	client *dockerclient.DockerClient
}

type Driver struct {
	val int
	dockerer
}

func NewDriver() (*Driver, error) {
	docker, err := dockerclient.NewDockerClient("unix:///var/run/docker.sock", nil)
	if err != nil {
		return nil, fmt.Errorf("could not connect to docker: %s", err)
	}

	d := &Driver{
		dockerer: dockerer{
			client: docker,
		},
	}
	return d, nil
}

func (d Driver) AllocateNetwork(req *network.AllocateNetworkRequest) (*network.AllocateNetworkResponse, error) {
	log.Debugf("Allocate network request: %+v", req)
	res := &network.AllocateNetworkResponse{
		Options: make(map[string]string),
	}
	return res, nil
}

func (d Driver) CreateEndpoint(req *network.CreateEndpointRequest) (*network.CreateEndpointResponse, error) {
	log.Debugf("Create network request: %+v", req)
	fmt.Println("Create network request", req)
	return nil, nil
}

func (d Driver) DeleteEndpoint(req *network.DeleteEndpointRequest) error {
	return nil
}

func (d Driver) CreateNetwork(req *network.CreateNetworkRequest) error {
	return nil
}

func (d Driver) DeleteNetwork(req *network.DeleteNetworkRequest) error {
	return nil
}

func (d Driver) DiscoverDelete(req *network.DiscoveryNotification) error {
	return nil
}

func (d Driver) DiscoverNew(notif *network.DiscoveryNotification) error {
	return nil
}

func (d Driver) EndpointInfo(req *network.InfoRequest) (*network.InfoResponse, error) {
	return nil, nil
}

func (d Driver) FreeNetwork(req *network.FreeNetworkRequest) error {
	return nil
}

func (d Driver) GetCapabilities() (*network.CapabilitiesResponse, error) {
	res := &network.CapabilitiesResponse{
		Scope: network.LocalScope,
	}
	return res, nil
}

func (d Driver) Join(req *network.JoinRequest) (*network.JoinResponse, error) {
	return nil, nil
}

func (d Driver) Leave(req *network.LeaveRequest) error {
	return nil
}

func (d Driver) ProgramExternalConnectivity(req *network.ProgramExternalConnectivityRequest) error {
	return nil
}

func (d Driver) RevokeExternalConnectivity(req *network.RevokeExternalConnectivityRequest) error {
	return nil
}
