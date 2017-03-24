#!/usr/bin/env bash

set -e
set -o xtrace

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

docker network create --attachable --driver ovn test1
