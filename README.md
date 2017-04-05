# OVN plugin for libnetwork

[![Build Status](https://travis.ibm.com/kangh/libnetwork-ovn-plugin.svg?token=z7j3APJPtnqnWXsjYFyp&branch=master)](https://travis.ibm.com/kangh/libnetwork-ovn-plugin)

This repository contains an OVN plugin for libnetwork. The implementation is
based on the [remote](https://github.com/docker/libnetwork/blob/f6ce0ce8bfc5e3f0c96835b10949cf13591a1708/docs/remote.md) driver of libnetwork. Some idea and implementations refer to two precedents:
[**docker-ovs-plugin**](https://github.com/gopher-net/docker-ovs-plugin) and [**OVN with Docker**] (http://docs.openvswitch.org/en/latest/howto/docker/).

### QuickStart Instructions

The quickstart instructions describe how to start the plugin in **overlay** mode,
which means the logical networks and containers are created directly on the hosts.

1. Install open vswitch and ovn
=============

There are many ways of installing OVS and OVN. In this instruction, we will install the use space OVS and OVN (**v2.7.0**) components by docker containers.

*Note*: OVS kernel module must be installed on the host or enabe the user mode OVS bridge (e.g., the [travis-ci script](https://github.ibm.com/kangh/libnetwork-ovn-plugin/blob/3ed4922c234a982a4f9ab541e5a868177df28274/run-integration-tests.sh#L20)).

Compile and install OVN kernel module on the host:

    wget http://openvswitch.org/releases/openvswitch-2.7.0.tar.gz
    ./configure --prefix=/usr --localstatedir=/var  --sysconfdir=/etc --with-linux=/lib/modules/`uname -r`/build
    make -j4
    rmmod openvswitch
    modprobe nf_nat_ipv6
    modprobe gre
    insmod ./datapath/linux/*.ko


Start the OVS and OVN processes using the script in this repository:

*Note*: Edit the following script for your own environment


    go get github.com:huikang/libnetwork-ovn-plugin.git
    ./scripts/start-ovn-central.sh

To very the host has been connected to the OVN centralized controller, type

    docker exec ovn-central ovn-sbctl show

2.  Start plugins
=============

Start libnetwork OVN plugin:

        make
        ./bin/libnetwork-ovn-plugin


3. Test the OVN-managed network for containers
=============

Create a network:

    docker network create --driver ovn --attachable --subnet=10.0.0.0/24 net1

Create two containers and assign them the network:

    docker run -d --net=net1 --name c1 busybox sleep 100000
    docker run -d --net=net1 --name c2 busybox sleep 100000

Enter the containers and verify the connectivity by ping the other one. The
endpoint of the containers are added to the OVN's southbound database:

    # docker exec ovn-central ovn-sbctl show
    Chassis "56b2a16e-c80a-4550-9cfe-c8bc1320bc2c"
    hostname: "dockerDev06"
    Encap geneve
       ip: "127.0.0.1"
       options: {csum="true"}
    Port_Binding "br46bfc-6d6a1"
    Port_Binding "br46bfc-2e0ac"
