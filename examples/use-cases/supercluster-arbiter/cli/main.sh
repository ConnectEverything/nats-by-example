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
    {name: rg1, urls: [
      nats://localhost:7222,
      nats://localhost:7223,
      nats://localhost:7224,
    ]},
    {name: rg2, urls: [
      nats://localhost:7225,
      nats://localhost:7226,
      nats://localhost:7227,
    ]},
  ]
}
EOF

# Partial config to be included in each server config in
# each cluster. This is the connection back to the arbiter.
cat <<- EOF > gateway-shared.conf
gateways: [
  {name: arbiter, urls: [
    nats://localhost:7228,
  ]},
]
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

# Define the server configs for region 1 cluster.
cat <<- EOF > "rg1-az1.conf"
server_name: rg1-az1
server_tags: [az:1]
port: 4222
http_port: 8222
include cluster-shared.conf
cluster: {
  name: rg1
  port: 6222
  routes: [
    nats-route://127.0.0.1:6222,
    nats-route://127.0.0.1:6223,
    nats-route://127.0.0.1:6224,
  ]
}
gateway {
  name: rg1
  port: 7222
  include gateway-shared.conf
}
EOF

cat <<- EOF > "rg1-az2.conf"
server_name: rg1-az2
server_tags: [az:2]
port: 4223
include cluster-shared.conf
cluster: {
  name: rg1
  port: 6223
  routes: [
    nats-route://127.0.0.1:6222,
    nats-route://127.0.0.1:6223,
    nats-route://127.0.0.1:6224,
  ]
}
gateway {
  name: rg1
  port: 7223
  include gateway-shared.conf
}
EOF

cat <<- EOF > "rg1-az3.conf"
server_name: rg1-az3
server_tags: [az:3]
port: 4224
include cluster-shared.conf
cluster: {
  name: rg1
  port: 6224
  routes: [
    nats-route://127.0.0.1:6222,
    nats-route://127.0.0.1:6223,
    nats-route://127.0.0.1:6224,
  ]
}
gateway {
  name: rg1
  port: 7224
  include gateway-shared.conf
}
EOF

# Server configs for region 2 cluster.
cat <<- EOF > "rg2-az1.conf"
server_name: rg2-az1
server_tags: [az:1]
port: 4225
http_port: 8223
include cluster-shared.conf
cluster: {
  name: rg2
  port: 6225
  routes: [
    nats-route://127.0.0.1:6225,
    nats-route://127.0.0.1:6226,
    nats-route://127.0.0.1:6227,
  ]
}
gateway {
  name: rg2
  port: 7225
  include gateway-shared.conf
}
EOF

cat <<- EOF > "rg2-az2.conf"
server_name: rg2-az2
server_tags: [az:2]
port: 4226
include cluster-shared.conf
cluster: {
  name: rg2
  port: 6226
  routes: [
    nats-route://127.0.0.1:6225,
    nats-route://127.0.0.1:6226,
    nats-route://127.0.0.1:6227,
  ]
}
gateway {
  name: rg2
  port: 7226
  include gateway-shared.conf
}
EOF

cat <<- EOF > "rg2-az3.conf"
server_name: rg2-az3
server_tags: [az:3]
port: 4227
include cluster-shared.conf
cluster: {
  name: rg2
  port: 6227
  routes: [
    nats-route://127.0.0.1:6225,
    nats-route://127.0.0.1:6226,
    nats-route://127.0.0.1:6227,
  ]
}
gateway {
  name: rg2
  port: 7227
  include gateway-shared.conf
}
EOF

# Start a server for each configuration and sleep a second in
# between so the seeds can startup and get healthy.
echo "Starting arbiter"
nats-server -c arbiter.conf > /dev/null 2>&1 &

for c in $(ls rg*.conf); do
  echo "Starting server ${c%.*}"
  nats-server -c $c > /dev/null 2>&1 &
  sleep 1
done

# Wait until the servers up and healthy.
echo 'Cluster 1 healthy?'
curl --fail --silent \
  --retry 5 \
  --retry-delay 1 \
  http://localhost:8222/healthz; echo

echo 'Cluster 2 healthy?'
curl --fail --silent \
  --retry 5 \
  --retry-delay 1 \
  http://localhost:8223/healthz; echo

echo 'Arbiter healthy?'
curl --fail --silent \
  --retry 5 \
  --retry-delay 1 \
  http://localhost:8224/healthz; echo

# List the servers, show the JetStream report, and show the info
# of an arbitrary server in region 2.
nats --user sys --password sys server list
nats --user sys --password sys server report jetstream
nats --user sys --password sys server info rg2-az1

# Connect to cluster 1 (port 4222), but request that a stream be
# added to the cluster 2.
nats --server nats://localhost:4222 \
     --user user \
     --password user stream add \
  --cluster=rg2 \
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
