# OVN plugin for libnetwork

[![Build Status](https://travis-ci.org/huikang/libnetwork-ovn-plugin.svg?branch=master)](https://travis-ci.org/huikang/libnetwork-ovn-plugin)

This repository contains an OVN plugin for libnetwork. The implementation is
based on the [remote](https://github.com/docker/libnetwork/blob/f6ce0ce8bfc5e3f0c96835b10949cf13591a1708/docs/remote.md) driver of libnetwork. Some idea and implementations refer to two precedents:
[**docker-ovs-plugin**](https://github.com/gopher-net/docker-ovs-plugin) and [**OVN with Docker**](http://docs.openvswitch.org/en/latest/howto/docker/).

### QuickStart Instructions

The quickstart instructions describe how to start the plugin in **overlay** mode,
which means the logical networks and containers are created directly on the hosts.
The following steps sets up the OVN plugin for a single docker host; Refer
to the **[multihost](https://github.com/huikang/libnetwork-ovn-plugin/blob/master/docs/multihost-ovn.md)** for setting up OVN plugin for multi-host docker cluster.

## Start docker daemon with a global data store

The OVN plugin requires a distributed datastore to support global data scope.
Therefore, the docker daemon must start with a global data store.

*Note*: since docker swarm mode does not support remote network driver, you can
choose consul or etcd as the backend data store. For example, the following command
bootstrap a single-node consul cluster:

    wget https://releases.hashicorp.com/consul/0.8.3/consul_0.8.3_linux_amd64.zip
    ./consul agent -server -bootstrap -data-dir /tmp/consul/ \
                   -advertise=<IP address of eth0 or eth1> -client=0.0.0.0

Restart the docker daemon and connect to the consul cluster:

    systemctl stop docker
    dockerd -H tcp://127.0.0.1:2375 -H unix:///var/run/docker.sock \
            --cluster-store=consul://CONSULIP:8500 --cluster-advertise=eth0:2376

## Install Open vSwitch and Ovn

There are many ways of installing OVS and OVN. In this instruction, we will install the use space OVS and OVN (**v2.7.0**) components by docker containers.

*Note*: OVS kernel module must be installed on the host or enable the user mode OVS bridge (e.g., the [travis-ci script](https://github.com/huikang/libnetwork-ovn-plugin/blob/6e5f911c94a59a589ce4456129524dd81a480ff4/run-integration-tests.sh#L60)).

Compile and install OVN kernel module on the host:

    wget http://openvswitch.org/releases/openvswitch-2.7.0.tar.gz
    ./configure --prefix=/usr --localstatedir=/var  --sysconfdir=/etc \
                --with-linux=/lib/modules/`uname -r`/build
    make -j4
    rmmod openvswitch
    modprobe nf_nat_ipv6
    modprobe gre
    insmod ./datapath/linux/openvswitch.ko
    insmod ./datapath/linux/vport-geneve.ko

The **vport-geneve** module must be installed because the default geneve dose not work with
the OVS 2.7.0. Also you may need installing other compiled modules.

Start the OVS and OVN processes using the script in this repository:

*Note*: Edit the following script for your own environment

    go get github.com/huikang/libnetwork-ovn-plugin
    ./scripts/start-ovn.sh -t aio

*Note*: the above command uses the script to start an all-in-one mode OVN. Refer
to the [multihost](https://github.com/huikang/libnetwork-ovn-plugin/blob/master/docs/multihost-ovn.md) for setting up docker cluster.

To very the host has been connected to the OVN centralized controller, type

    docker exec aio ovn-sbctl show

Also verify that the br-int is created with correct kernel module by:

    docker exec aio ovs-ofctl dump-flows br-int

The above command should return with no error.

## Start plugins

Start libnetwork OVN plugin:

        make
        ./bin/libnetwork-ovn-plugin


## Test the OVN-managed network for containers

Create a network:

    docker network create --driver ovn --attachable --subnet=10.0.0.0/24 net1

Create two containers and assign them the network:

    docker run -d --net=net1 --name c1 busybox sleep 100000
    docker run -d --net=net1 --name c2 busybox sleep 100000

Enter the containers and verify the connectivity by ping the other one. The
endpoint of the containers are added to the OVN's southbound database:

    # docker exec ovn-central | aio ovn-sbctl show
    Chassis "56b2a16e-c80a-4550-9cfe-c8bc1320bc2c"
    hostname: "dockerDev06"
    Encap geneve
       ip: "127.0.0.1"
       options: {csum="true"}
    Port_Binding "br46bfc-6d6a1"
    Port_Binding "br46bfc-2e0ac"
