Bootstrap consul agent on one node

     ./consul agent -server -bootstrap -data-dir /tmp/consul/ -client CONSULNODEIP

``CONSULNODEIP`` is the IP address by which other docker hosts can reach the consul node.
If the node has multiple IP addresses, pick one that is reachable from others.

On all docker hosts, the docker daemon should be connected to the consul cluster.
Depending on the OS, there are different ways to configure the docker daemon.
On the systemd managed OS, for example, edit the docker service file
``/lib/systemd/system/docker.service``


``
[Service]
ExecStart=/usr/bin/docker daemon --cluster-store=consul://${CONSULNODEIP}:8500 --cluster-advertise=eth0:2376
``


Then, restart the docker daemon

``shell
systemctl daemon-reload
systemctl enable docker
systemctl restart docker
``
