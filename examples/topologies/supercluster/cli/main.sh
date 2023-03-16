#!/bin/sh

set -euo pipefail

# Define the system account to be included by all configurations.
cat <<- EOF > sys.conf
accounts: {
  \$SYS: {
    users: [{user: sys, password: sys}]
  }
}
EOF

# Create the *east* and *west* single-node server configurations
# declaring gateway connections for each. By default, the gateway
# name will be used as the cluster name even if the cluster block
# is not specified. Of course, if this were a real cluster with
# routes defined, this would need to be defined as well.
cat <<- EOF > east.conf
port: 4222
http_port: 8222
server_name: n1

include sys.conf

gateway: {
  name: east,
  port: 7222,
  gateways: [
    {name: "east", urls: ["nats://0.0.0.0:7222"]},
    {name: "west", urls: ["nats://0.0.0.0:7223"]},
  ]
}
EOF

cat <<- EOF > west.conf
port: 4223
http_port: 8223
server_name: n2

include sys.conf

gateway: {
  name: west,
  port: 7223,
  gateways: [
    {name: "east", urls: ["nats://0.0.0.0:7222"]},
    {name: "west", urls: ["nats://0.0.0.0:7223"]},
  ]
}
EOF


# Start the servers and sleep for a few seconds to startup.
nats-server -c east.conf > /dev/null 2>&1 &
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


# Save a couple NATS CLI contexts for convenience.
nats context save east \
  --server "nats://localhost:4222"

nats context save east-sys \
  --server "nats://localhost:4222" \
  --user sys \
  --password sys

nats context save west \
  --server "nats://localhost:4223"


# Show the server list which will indicate the clusters and
# gateway connections.
nats --context east-sys server list

# Start a service running in east.
nats --context east reply 'greet' 'hello from east' &

sleep 1

# Send a request with a client connected to west.
nats --context west request 'greet' ''
