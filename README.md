# libnetwork-ovn-plugin
OVN plugin for libnetwork

[![Build Status](https://travis.ibm.com/kangh/libnetwork-ovn-plugin.svg?token=z7j3APJPtnqnWXsjYFyp&branch=master)](https://travis.ibm.com/kangh/libnetwork-ovn-plugin)

### QuickStart Instructions

The quickstart instructions describe how to start the plugin in **overlay** mode.

1. Start OVN centralized controller

The OVN centralized controller includes a northbound database, a southbound database, and an ovn-northd process. The three processes run in a single container::

        ./scripts/start-ovn-central.sh


2. Start libnetwork OVN plugin

        make
        ./bin/libnetwork-ovn-plugin

Create a network::

        docker network create \
                       --attachable \
                       --driver ovn \
                       --subnet=10.10.1.0/24 \
                       --gateway=10.10.1.1 ovn-net
