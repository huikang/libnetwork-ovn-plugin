Bootstrap consul agent on one node

     wget https://releases.hashicorp.com/consul/0.8.0/consul_0.8.0_linux_amd64.zip
     ./consul agent -server -bootstrap -data-dir /tmp/consul/ -client CONSULNODEIP -advertise CONSULNODEIP

``CONSULNODEIP`` is the IP address by which other docker hosts can reach the consul node.
If the node has multiple IP addresses, pick one that is reachable from others.

On *all docker hosts*, the docker daemon should be connected to the consul cluster.
Depending on the OS, there are different ways to configure the docker daemon.
On the systemd managed OS, for example, edit the docker service file
``/lib/systemd/system/docker.service``


``
[Service]
ExecStart=/usr/bin/docker daemon --cluster-store=consul://${CONSULNODEIP}:8500 --cluster-advertise=eth0:2376
``

Then, restart the docker daemon

``
systemctl daemon-reload
systemctl enable docker
systemctl restart docker
``

To see the docker host has registered itself to the cluster, run:

    ./consul kv export -http-addr ${CONSULNODEIP}:8500

## Start the ovn centralized node

    go get github.com/huikang/libnetwork-ovn-plugin
    ./scripts/start-ovn.sh -t aio -r ${CENTRALNODEIP} -s ${CENTRALNODEIP}

## Start the ovn controller node on all nodes

    go get github.com/huikang/libnetwork-ovn-plugin
    ./scripts/start-ovn.sh -t ovn-controller -r ${CENTRALNODEIP} -s ${CONTROLLIP}

Start the ovn plugin and specify the IP of centralized node because the plugin needs to
connect to the OVN northoubnd database

    ./bin/libnetwork-ovn-plugin -r ${CENTRALNODEIP}
