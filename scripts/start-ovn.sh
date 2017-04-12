#!/bin/bash -e

# NODETYPE specifies the OVN node type to be started
NODETYPE=
# OVNIP and SELFIP are needed by the ovn-controller node
OVNREMOTEIP=
SELFIP=

cidovs=

function helpme {
    printf "Usage: $0 -t [ ovn-central | ovn-controller | aio ] -r [IP of OVN central node] -s [self IP] \n"
    exit 1
}

function valid_ip {
    local  ip=$1
    local  stat=1

    if [[ $ip =~ ^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$ ]]; then
        OIFS=$IFS
        IFS='.'
        ip=($ip)
        IFS=$OIFS
        [[ ${ip[0]} -le 255 && ${ip[1]} -le 255 \
            && ${ip[2]} -le 255 && ${ip[3]} -le 255 ]]
        stat=$?
    fi
    return $stat
}

function configure_ovn_central {
    echo "Start OVN centralized node"
    echo "Starting ovn NB, SB, and ovn-northd"

    docker exec $cidovs ovsdb-tool create /etc/openvswitch/ovnnb_db.db /usr/local/share/openvswitch/ovn-nb.ovsschema
    docker exec $cidovs ovsdb-tool create /etc/openvswitch/ovnsb_db.db /usr/local/share/openvswitch/ovn-sb.ovsschema
    docker exec $cidovs supervisorctl start ovsdb-server-nb
    docker exec $cidovs supervisorctl start ovsdb-server-sb
    docker exec $cidovs supervisorctl start ovn-northd
}

function configure_ovn_controller
{
    local  ovnremote=$1
    local  self=$2
    echo "Starting ovn-controller $ovnremote $self"
    sudo modprobe geneve

    docker exec $cidovs ovsdb-tool create /etc/openvswitch/vtep.db /usr/local/share/openvswitch/vtep.ovsschema
    docker exec $cidovs supervisorctl stop ovsdb-server
    docker exec $cidovs supervisorctl start ovsdb-server-vtep

    docker exec $cidovs sh -c " ovs-vsctl set Open_vSwitch . external_ids:ovn-remote='tcp:${ovnremote}:6642' external_ids:ovn-nb='tcp:${ovnremote}:6641' external_ids:ovn-encap-ip=${self} external_ids:ovn-encap-type='geneve'"
    docker exec $cidovs supervisorctl start ovn-controller
}

# Parse arguments
while getopts ":t:r:s:a" opt; do
    case $opt in
    t)
        echo "${OPTARG}"
        echo "-t was triggered, Parameter: $OPTARG" >&2
        NODETYPE="${OPTARG}"
	;;
    r)
        echo "${OPTARG}"
        echo "-r was triggered, Parameter: $OPTARG" >&2
        OVNREMOTEIP="${OPTARG}"
        if valid_ip $OVNREMOTEIP; then
            echo "Connect to remote OVN $OVNREMOTEIP"
        else
            echo "Invalid remote IP: $OVNREMOTEIP"
            exit 1
        fi
        ;;
    s)
        echo "${OPTARG}"
        echo "-r was triggered, Parameter: $OPTARG" >&2
        SELFIP="${OPTARG}"
        if valid_ip $SELFIP; then
            echo "Self IP: $SELFIP"
        else
            echo "Invalid self IP: $SELFIP"
            exit 1
        fi
        ;;
    \?)
        echo "Invalid option: -$OPTARG" >&2
        helpme
        exit 1
        ;;
    :)
        echo "Option -$OPTARG requires an argument." >&2
        exit 1
        ;;
    'h'|'?')
        helpme
        exit 1
  esac
done
shift $((${OPTIND} - 1))

OVNREMOTEIP=${OVNREMOTEIP:-"127.0.0.1"}
SELFIP=${SELFIP:-"127.0.0.1"}
CNAME=${NODETYPE}

echo "OVNREMOTE: ${OVNREMOTEIP}"
echo "SELFIP: ${SELFIP}"

# Start the basic OVS container
cidovs=$(docker run -itd --net=host --privileged -v /lib/modules:/lib/modules --name ${CNAME} huikang/openvswitch:2.7.0)
if [ $? -eq 0 ]; then
    echo "Started openvswitch container $cidovs"
else
    echo "Error starting OVS container"
    exit 1
fi
sleep 2

# Configure OVN container: ovn-central, ovn-controller, or aio
case $NODETYPE in
  aio)
    echo "[ $NODETYPE ] mode"
    configure_ovn_central
    configure_ovn_controller $OVNREMOTEIP $SELFIP
    ;;
  ovn-central)
    echo "[ $NODETYPE ] mode"
    configure_ovn_central
    ;;
  ovn-controller)
    echo "[ $NODETYPE ] mode"
    configure_ovn_controller $OVNREMOTEIP $SELFIP
    ;;
  *)
    echo "Invalid node type"
    helpme
    exit 1
    ;;
esac
