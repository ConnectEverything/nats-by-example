#!/bin/sh

set -euo pipefail

# Define the system account to be included by all configurations.
cat <<- EOF > sys.conf
accounts: {
  SYS: {
    users: [{user: sys, password: pass}]
  }
}

system_account: SYS
EOF

# Create the *east*, *central*, and *west* server configurations.
# A requirement of JetStream is to have a cluster block with routes
# defined, even for single node clusters.
#
# In a production deployment, at least three nodes per cluster
# are recommended which supports creating R3 streams. In this
# test setup, only R1 streams can be created since streams replicas
# do not cross gateway connections.
#
# Similarity, across a supercluster, an odd number of servers must
# exist to support reaching consensus for the meta group leader.
cat <<- EOF > east.conf
port: 4222
http_port: 8222
server_name: n1

include sys.conf

jetstream: {}

cluster: {
  name: east,
  port: 6222,
  routes: [
    "nats-route://0.0.0.0:6222"
  ],
}

gateway: {
  name: "east",
  port: 7222,
  gateways: [
    {name: "east", urls: ["nats://0.0.0.0:7222"]},
    {name: "central", urls: ["nats://0.0.0.0:7223"]},
    {name: "west", urls: ["nats://0.0.0.0:7224"]},
  ]
}
EOF

cat <<- EOF > central.conf
port: 4223
http_port: 8223
server_name: n2

include sys.conf

jetstream: {}

cluster: {
  name: central,
  port: 6223,
  routes: [
    "nats-route://0.0.0.0:6223"
  ],
}

gateway: {
  name: "central",
  port: 7223,
  gateways: [
    {name: "east", urls: ["nats://0.0.0.0:7222"]},
    {name: "central", urls: ["nats://0.0.0.0:7223"]},
    {name: "west", urls: ["nats://0.0.0.0:7224"]},
  ]
}
EOF


cat <<- EOF > west.conf
port: 4224
http_port: 8224
server_name: n3

include sys.conf

jetstream: {}

cluster: {
  name: west,
  port: 6224,
  routes: [
    "nats-route://0.0.0.0:6224"
  ],
}

gateway: {
  name: "west",
  port: 7224,
  gateways: [
    {name: "east", urls: ["nats://0.0.0.0:7222"]},
    {name: "central", urls: ["nats://0.0.0.0:7223"]},
    {name: "west", urls: ["nats://0.0.0.0:7224"]},
  ]
}
EOF


# Start the servers and sleep for a few seconds to startup.
nats-server -c east.conf > /dev/null 2>&1 &
nats-server -c central.conf > /dev/null 2>&1 &
nats-server -c west.conf > /dev/null 2>&1 &

sleep 3


# Wait until the servers are healthy.
curl --fail --silent \
  --retry 5 \
  --retry-delay 1 \
  http://localhost:8222/healthz > /dev/null

curl --fail --silent \
  --retry 5 \
  --retry-delay 1 \
  http://localhost:8223/healthz > /dev/null

curl --fail --silent \
  --retry 5 \
  --retry-delay 1 \
  http://localhost:8224/healthz > /dev/null


# Save a couple NATS CLI contexts for convenience.
nats context save east \
  --server "nats://localhost:4222" > /dev/null

nats context save east-sys \
  --server "nats://localhost:4222" \
  --user sys \
  --password pass > /dev/null

nats context save west \
  --server "nats://localhost:4223" > /dev/null


# Show the server list which will indicate the clusters and
# gateway connections as well as the JetStream server report.
nats --context east-sys server list
nats --context east-sys server report jetstream

# Start a service running in east.
nats --context east reply 'greet' 'hello from east' &

sleep 1

# Send a request with a client connected to west.
nats --context west request 'greet' ''
