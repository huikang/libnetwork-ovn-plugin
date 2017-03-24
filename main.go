package main

import (
	"fmt"
	_ "os"

	"github.com/docker/go-plugins-helpers/network"

	"github.ibm.com/kangh/libnetwork-ovn-plugin/ovn"
)

type MyNetworkDriver struct {
	val int
}

func main() {
	fmt.Println("Libnetwork ovn plugin")
	fmt.Println(ovn.DefaultRoute)

	d := MyNetworkDriver{}

	h := network.NewHandler(d)
	h.ServeTCP("ovn", ":8087", nil)
}

func (d MyNetworkDriver) AllocateNetwork(req *network.AllocateNetworkRequest) (*network.AllocateNetworkResponse, error) {
	return nil, nil
}

func (d MyNetworkDriver) CreateEndpoint(req *network.CreateEndpointRequest) (*network.CreateEndpointResponse, error) {
	return nil, nil
}

func (d MyNetworkDriver) DeleteEndpoint(req *network.DeleteEndpointRequest) error {
	return nil
}

func (d MyNetworkDriver) CreateNetwork(req *network.CreateNetworkRequest) error {
	return nil
}

func (d MyNetworkDriver) DeleteNetwork(req *network.DeleteNetworkRequest) error {
	return nil
}

func (d MyNetworkDriver) DiscoverDelete(req *network.DiscoveryNotification) error {
	return nil
}

func (d MyNetworkDriver) DiscoverNew(notif *network.DiscoveryNotification) error {
	return nil
}

func (d MyNetworkDriver) EndpointInfo(req *network.InfoRequest) (*network.InfoResponse, error) {
	return nil, nil
}

func (d MyNetworkDriver) FreeNetwork(req *network.FreeNetworkRequest) error {
	return nil
}

func (d MyNetworkDriver) GetCapabilities() (*network.CapabilitiesResponse, error) {
	return nil, nil
}

func (d MyNetworkDriver) Join(req *network.JoinRequest) (*network.JoinResponse, error) {
	return nil, nil
}

func (d MyNetworkDriver) Leave(req *network.LeaveRequest) error {
	return nil
}

func (d MyNetworkDriver) ProgramExternalConnectivity(req *network.ProgramExternalConnectivityRequest) error {
	return nil
}

func (d MyNetworkDriver) RevokeExternalConnectivity(req *network.RevokeExternalConnectivityRequest) error {
	return nil
}
