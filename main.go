package main

import (
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/go-plugins-helpers/network"
	"github.com/huikang/libnetwork-ovn-plugin/ovn"
	"gopkg.in/urfave/cli.v1"
)

const (
	version = "0.1"
)

func main() {
	app := cli.NewApp()
	app.Name = "Libnetwork OVN plugin"
	app.Usage = "./libnetwork-ovn-plugin"
	app.Version = version

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug, d",
			Usage: "enabling debugging",
		},
	}

	app.Action = pluginServer
	app.Run(os.Args)
}

func pluginServer(c *cli.Context) error {

	if c.Bool("debug") {
		log.SetLevel(log.DebugLevel)
	}

	d, err := ovn.NewDriver()
	if err != nil {
		panic(err)
	}

	h := network.NewHandler(d)
	h.ServeUnix(ovn.DriverName, 0)
	return nil
}
