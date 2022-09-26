#!/bin/bash

set -euo pipefail

# Define the arbiter config which is a single node that
# defines gateway connections between each of the clusters.
cat <<- EOF > arbiter.conf
server_name: arbiter
port: 4228
http_port: 8224

gateway {
  name: arbiter
  port: 7228
  gateways: [
    {name: east, urls: [
      nats://localhost:7222,
      nats://localhost:7223,
      nats://localhost:7224,
    ]},
    {name: west, urls: [
      nats://localhost:7225,
      nats://localhost:7226,
      nats://localhost:7227,
    ]},
  ]
}
EOF

# General shared config for each server in the clusters.
# JetStream enabled with the unique tag as well as basic
# accounts.
cat <<- EOF > cluster-shared.conf
jetstream: {
  unique_tag: "az:"
}
accounts: {
  '\$SYS': {
    users: [{user: sys, password: sys}]
  }
  APP: {
    jetstream: true
    users: [{user: user, password: user}]
  }
}
EOF

# Define the server configs for east cluster.
cat <<- EOF > "east-az1.conf"
server_name: east-az1
server_tags: [az:1]
port: 4222
http_port: 8222
include cluster-shared.conf
cluster: {
  name: east
  port: 6222
  routes: [
    nats-route://127.0.0.1:6222,
    nats-route://127.0.0.1:6223,
    nats-route://127.0.0.1:6224,
  ]
}
gateway {
  name: east
  port: 7222
}
EOF

cat <<- EOF > "east-az2.conf"
server_name: east-az2
server_tags: [az:2]
port: 4223
include cluster-shared.conf
cluster: {
  name: east
  port: 6223
  routes: [
    nats-route://127.0.0.1:6222,
    nats-route://127.0.0.1:6223,
    nats-route://127.0.0.1:6224,
  ]
}
gateway {
  name: east
  port: 7223
}
EOF

cat <<- EOF > "east-az3.conf"
server_name: east-az3
server_tags: [az:3]
port: 4224
include cluster-shared.conf
cluster: {
  name: east
  port: 6224
  routes: [
    nats-route://127.0.0.1:6222,
    nats-route://127.0.0.1:6223,
    nats-route://127.0.0.1:6224,
  ]
}
gateway {
  name: east
  port: 7224
}
EOF

# Server configs for west cluster.
cat <<- EOF > "west-az1.conf"
server_name: west-az1
server_tags: [az:1]
port: 4225
http_port: 8223
include cluster-shared.conf
cluster: {
  name: west
  port: 6225
  routes: [
    nats-route://127.0.0.1:6225,
    nats-route://127.0.0.1:6226,
    nats-route://127.0.0.1:6227,
  ]
}
gateway {
  name: west
  port: 7225
}
EOF

cat <<- EOF > "west-az2.conf"
server_name: west-az2
server_tags: [az:2]
port: 4226
include cluster-shared.conf
cluster: {
  name: west
  port: 6226
  routes: [
    nats-route://127.0.0.1:6225,
    nats-route://127.0.0.1:6226,
    nats-route://127.0.0.1:6227,
  ]
}
gateway {
  name: west
  port: 7226
}
EOF

cat <<- EOF > "west-az3.conf"
server_name: west-az3
server_tags: [az:3]
port: 4227
include cluster-shared.conf
cluster: {
  name: west
  port: 6227
  routes: [
    nats-route://127.0.0.1:6225,
    nats-route://127.0.0.1:6226,
    nats-route://127.0.0.1:6227,
  ]
}
gateway {
  name: west
  port: 7227
}
EOF

# Start all of the servers.
echo 'Starting the servers...'
nats-server -c arbiter.conf > /dev/null 2>&1 &
nats-server -c east-az1.conf > /dev/null 2>&1 &
nats-server -c east-az2.conf > /dev/null 2>&1 &
nats-server -c east-az3.conf > /dev/null 2>&1 &
nats-server -c west-az1.conf > /dev/null 2>&1 &
nats-server -c west-az2.conf > /dev/null 2>&1 &
nats-server -c west-az3.conf > /dev/null 2>&1 &

sleep 3

# Wait until the servers up and healthy.
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

# List the servers, show the JetStream report, and show the info
# of an arbitrary server in west.
nats --user sys --password sys server list
nats --user sys --password sys server report jetstream
nats --user sys --password sys server info west-az1

# Connect to east (port 4222), but request that a stream be
# added to the west.
nats stream add \
  --server nats://localhost:4222 \
  --user user \
  --password user \
  --cluster=west \
  --retention=limits \
  --storage=file \
  --replicas=3 \
  --discard=old \
  --dupe-window=2m \
  --max-age=-1 \
  --max-msgs=-1 \
  --max-bytes=-1 \
  --max-msg-size=-1 \
  --max-msgs-per-subject=-1 \
  --max-consumers=-1 \
  --allow-rollup \
  --no-deny-delete \
  --no-deny-purge \
  --subjects="orders.*" \
  ORDERS

# Report out the streams to confirm placement.
nats --user user --password user stream report
