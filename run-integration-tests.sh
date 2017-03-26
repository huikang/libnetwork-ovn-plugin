#!/usr/bin/env bash

set -e
set -o xtrace

docker version

echo "Run integration test"
sudo service docker restart

source ./scripts/start-ovn-central.sh

docker exec $cid sh -c "ovn-nbctl show"
docker exec $cid sh -c "ovn-sbctl show"

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
