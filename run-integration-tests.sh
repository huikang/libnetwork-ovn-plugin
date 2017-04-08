#!/usr/bin/env bash

set -e
set -o xtrace

# check if logical port exists in local ovsdb
function exist_logical_port {
    local ret=1
    sbkey=$1
    output=`docker exec ovn-central ovsdb-client dump Interface`
    echo $output | grep -q "$sbkey"
    if [ $? -eq 0 ]
    then
        echo "Found $sbkey in vswitch"
        ret=0
    else
        echo "No $sbkey in vswitch"
        ret=1
    fi
    return $ret
}

function get_sboxkey() {
    declare -n ret=$2
    cid=$1
    path=`docker inspect -f '{{json .NetworkSettings.SandboxKey}}' $cid `

    sbkey=$(basename "$path" "\"")
    ret=$sbkey
}

function setup_consul {
    wget https://releases.hashicorp.com/consul/0.5.2/consul_0.5.2_linux_amd64.zip
    unzip consul_0.5.2_linux_amd64.zip
    ./consul agent -server -bootstrap -data-dir /tmp/consul/ > /tmp/consul.log 2>&1 &
    sleep 1
}

docker version

echo "Run integration test"

setup_consul
#sudo echo 'DOCKER_OPTS="--cluster-store=consul:127.0.0.1:8500 --cluster-advertise=eth0:2376"' >> /etc/default/docker
sudo /etc/init.d/docker stop
sudo /usr/bin/dockerd -H tcp://127.0.0.1:2375 -H unix:///var/run/docker.sock ${DOCKER_OPTS} --raw-logs > /tmp/docker.log 2>&1 &
sleep 3
docker ps
ps -aux | grep docker

source ./scripts/start-ovn-central.sh

docker exec $cidovs sh -c "ovn-nbctl show"
docker exec $cidovs sh -c "ovn-sbctl show"

# NOTE(huikang): set user space openvswitch
sudo modprobe openvswitch
sudo rmmod openvswitch
sudo modprobe tun
docker exec $cidovs sh -c "ovs-vsctl set bridge br-int datapath_type=netdev"

function start_ovn_plugin {
    echo "Start ovn plugin"
    docker run \
	-d \
	--hostname=ovn-plugin \
	--name=ovn-plugin \
	--net=host \
	--privileged \
	-v $(pwd)/:/go/src/github.com/huikang/libnetwork-ovn-plugin \
	-w /go/src/github.com/huikang/libnetwork-ovn-plugin \
	-v /run:/run \
	mrjana/golang ./bin/libnetwork-ovn-plugin -d
}
start_ovn_plugin

NID=`docker network create --attachable --driver ovn --subnet=10.10.10.0/24 test1`
docker network inspect $NID
docker exec $cidovs ovn-nbctl show

# Create two containers and connect them to the logical switch. These two
# containera can ping each other
cid1=`docker run -d --net=$NID mrjana/golang sleep 100`
cid1IP=`docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $cid1`
docker exec $cid1 ip a
# check logical port is added to ovsdb
get_sboxkey $cid1 sbkey1
exist_logical_port $sbkey1
if [ $? -eq 0 ]
then
    echo "found logical port in ovsdb"
else
    echo "No logical port"
fi

# dump the openflow rules installed by ovn
docker exec $cidovs ovn-nbctl show
docker exec $cidovs ovs-ofctl dump-flows br-int

cid2=`docker run -d --net=$NID mrjana/golang sleep 100`
cid2IP=`docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $cid2`
docker inspect -f '{{json .NetworkSettings}}' $cid2 | jq
get_sboxkey $cid2 sbkey2

docker exec $cid1 ping $cid1IP -c 2
docker exec $cid1 ping $cid2IP -c 2
docker exec $cid2 ping $cid1IP -c 2
docker exec $cid2 ping $cid1IP -c 2

# clean up containers and make sure the associated resources are cleanup in
# OVSDB
docker rm -f $cid1
# Netlinks on the host
sudo ip a
set +o errexit
exist_logical_port $sbkey1
if [ $? -eq 0 ]
then
    echo "Found logical port in ovsdb, but container $cid1 has been removed"
    exit 1
else
    echo "No logical port"
fi
set -o errexit

# restart ovn plugin
docker rm -f ovn-plugin
start_ovn_plugin
docker rm -f $cid2

set +o errexit
exist_logical_port $sbkey2
if [ $? -eq 0 ]
then
    echo "Found logical port in ovsdb, but container $cid2 has been removed"
    exit 1
else
    echo "No logical port"
fi
set -o errexit
# Should not show any veth interface
sudo ip a
