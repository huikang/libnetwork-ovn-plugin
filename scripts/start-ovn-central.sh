#!/bin/sh -e

echo "Start OVN centralized controller"

cidovs=$(docker run -itd --net=host --privileged -v /lib/modules:/lib/modules --name ovn-central huikang/openvswitch:2.7.0)

if [ $? -eq 0 ]; then
    echo "Started openvswitch container $cidovs"
else
    echo FAIL
fi

echo "Starting ovn NB, SB, and ovn-northd"
sleep 2
docker exec $cidovs ovsdb-tool create /etc/openvswitch/ovnnb_db.db /usr/local/share/openvswitch/ovn-nb.ovsschema
docker exec $cidovs ovsdb-tool create /etc/openvswitch/ovnsb_db.db /usr/local/share/openvswitch/ovn-sb.ovsschema
docker exec $cidovs ovsdb-tool create /etc/openvswitch/vtep.db /usr/local/share/openvswitch/vtep.ovsschema
docker exec $cidovs supervisorctl start ovsdb-server-nb
docker exec $cidovs supervisorctl start ovsdb-server-sb
docker exec $cidovs supervisorctl start ovn-northd
docker exec $cidovs supervisorctl stop ovsdb-server
docker exec $cidovs supervisorctl start ovsdb-server-vtep

echo "Starting ovn-controller"
sudo modprobe geneve
docker exec $cidovs sh -c " ovs-vsctl set Open_vSwitch . external_ids:ovn-remote='tcp:127.0.0.1:6642' external_ids:ovn-nb='tcp:127.0.0.1:6641' external_ids:ovn-encap-ip=127.0.0.1 external_ids:ovn-encap-type='geneve'"

docker exec $cidovs supervisorctl start ovn-controller
