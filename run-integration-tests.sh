#!/usr/bin/env bash

set -e
set -o xtrace

docker version

echo "Run integration test"
sudo service docker restart

source ./scripts/start-ovn-central.sh

docker exec $cidovs sh -c "ovn-nbctl show"
docker exec $cidovs sh -c "ovn-sbctl show"

# NOTE(huikang): set user space openvswitch
sudo modprobe openvswitch
sudo rmmod openvswitch
sudo modprobe tun
docker exec $cidovs sh -c "ovs-vsctl set bridge br-int datapath_type=netdev"

echo "Start ovn plugin"
docker run \
       -d \
       --hostname=ovn-plugin \
       --name=ovn-plugin \
       --net=host \
       --privileged \
       -v $(pwd)/:/go/src/github.ibm.com/kangh/libnetwork-ovn-plugin \
       -w /go/src/github.ibm.com/kangh/libnetwork-ovn-plugin \
       -v /run:/run \
       mrjana/golang ./bin/libnetwork-ovn-plugin

NID=`docker network create --attachable --driver ovn --subnet=10.10.10.0/24 --gateway=10.10.10.1 test1`
docker network inspect $NID
docker exec $cidovs ovn-nbctl show

# Create two containers and connect them to the logical switch. These two
# containera can ping each other
cid1=`docker run -d --net=$NID mrjana/golang sleep 100`
docker inspect -f '{{json .NetworkSettings}}' $cid1 | jq
docker exec $cid1 ip a

# dump the openflow rules installed by ovn
docker exec $cidovs ovn-nbctl show
docker exec $cidovs ovs-ofctl dump-flows br-int

cid2=`docker run -d --net=$NID mrjana/golang sleep 100`
docker inspect -f '{{json .NetworkSettings}}' $cid2 | jq

docker exec $cid1 ping 10.10.10.2 -c 2
docker exec $cid1 ping 10.10.10.3 -c 2
docker exec $cid2 ping 10.10.10.2 -c 2
docker exec $cid2 ping 10.10.10.3 -c 2
