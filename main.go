package main

import (
	"fmt"
	_ "os"

	"github.com/docker/go-plugins-helpers/network"

	"github.ibm.com/kangh/libnetwork-ovn-plugin/ovn"
)

func main() {
	fmt.Println("Libnetwork ovn plugin")

	d, err := ovn.NewDriver()
	if err != nil {
		panic(err)
	}

	h := network.NewHandler(d)
	h.ServeUnix(ovn.DriverName, 0)
}
