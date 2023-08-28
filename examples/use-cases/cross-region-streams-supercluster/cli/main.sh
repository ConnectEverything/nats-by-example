#!/bin/bash

set -euo pipefail

NATS_URL="nats://localhost:4222"

# Create a shared configuration which enables JetStream and defines the
# `unique_tag` option which enforces all replicas for a given stream or
# consumer to be placed on nodes with different availability zones (AZ).
cat <<- EOF > regional-shared.conf
jetstream: {
  unique_tag: "az:"
}

accounts: {
  \$SYS: {
    users: [{user: sys, password: sys}]
  }
  APP: {
    jetstream: true
    users: [{user: user, password: user}]
  }
}
EOF

# The cross-region cluster defines a unique_tag on the cluster. Any AZ
# can be chosen as long as the region is unique.
cat <<- EOF > cross-region-shared.conf
jetstream: {
  unique_tag: "rg:"
}

accounts: {
  \$SYS: {
    users: [{user: sys, password: sys}]
  }
  APP: {
    jetstream: true
    users: [{user: user, password: user}]
  }
}
EOF

cat <<- EOF > gateway-routes.conf
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
  {name: rg3, urls: [
    nats://localhost:7228,
    nats://localhost:7229,
    nats://localhost:7230,
  ]},
  {name: xr, urls: [
    nats://localhost:7231,
    nats://localhost:7232,
    nats://localhost:7233,
  ]},
]
EOF

# Define the server configs for region 1 cluster.
cat <<- EOF > "rg1-az1.conf"
server_name: rg1-az1
server_tags: [az:1]
port: 4222
http_port: 8222
include regional-shared.conf
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
  include gateway-routes.conf
}
jetstream: {
  store_dir: /tmp/nats/storage/rg1-az1
}
EOF

cat <<- EOF > "rg1-az2.conf"
server_name: rg1-az2
server_tags: [az:2]
port: 4223
include regional-shared.conf
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
  include gateway-routes.conf
}
jetstream: {
  store_dir: /tmp/nats/storage/rg1-az2
}
EOF

cat <<- EOF > "rg1-az3.conf"
server_name: rg1-az3
server_tags: [az:3]
port: 4224
include regional-shared.conf
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
  include gateway-routes.conf
}
jetstream: {
  store_dir: /tmp/nats/storage/rg1-az3
}
EOF

# Server configs for region 2 cluster.
cat <<- EOF > "rg2-az1.conf"
server_name: rg2-az1
server_tags: [az:1]
port: 4225
http_port: 8223
include regional-shared.conf
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
  include gateway-routes.conf
}
jetstream: {
  store_dir: /tmp/nats/storage/rg2-az1
}
EOF

cat <<- EOF > "rg2-az2.conf"
server_name: rg2-az2
server_tags: [az:2]
port: 4226
include regional-shared.conf
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
  include gateway-routes.conf
}
jetstream: {
  store_dir: /tmp/nats/storage/rg2-az2
}
EOF

cat <<- EOF > "rg2-az3.conf"
server_name: rg2-az3
server_tags: [az:3]
port: 4227
include regional-shared.conf
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
  include gateway-routes.conf
}
jetstream: {
  store_dir: /tmp/nats/storage/rg2-az3
}
EOF

# Server configs for region 3 cluster.
cat <<- EOF > "rg3-az1.conf"
server_name: rg3-az1
server_tags: [az:1]
port: 4228
http_port: 8224
include regional-shared.conf
cluster: {
  name: rg3
  port: 6228
  routes: [
    nats-route://127.0.0.1:6228,
    nats-route://127.0.0.1:6229,
    nats-route://127.0.0.1:6230,
  ]
}
gateway {
  name: rg3
  port: 7228
  include gateway-routes.conf
}
jetstream: {
  store_dir: /tmp/nats/storage/rg3-az1
}
EOF

cat <<- EOF > "rg3-az2.conf"
server_name: rg3-az2
server_tags: [az:2]
port: 4229
include regional-shared.conf
cluster: {
  name: rg3
  port: 6229
  routes: [
    nats-route://127.0.0.1:6228,
    nats-route://127.0.0.1:6229,
    nats-route://127.0.0.1:6230,
  ]
}
gateway {
  name: rg3
  port: 7229
  include gateway-routes.conf
}
jetstream: {
  store_dir: /tmp/nats/storage/rg3-az2
}
EOF

cat <<- EOF > "rg3-az3.conf"
server_name: rg3-az3
server_tags: [az:3]
port: 4230
include regional-shared.conf
cluster: {
  name: rg3
  port: 6230
  routes: [
    nats-route://127.0.0.1:6228,
    nats-route://127.0.0.1:6229,
    nats-route://127.0.0.1:6230,
  ]
}
gateway {
  name: rg3
  port: 7230
  include gateway-routes.conf
}
jetstream: {
  store_dir: /tmp/nats/storage/rg3-az3
}
EOF

# Server configs for cross-region cluster. In this case, an arbitrary
# AZ is chosen per region. If may be desirable to have a six or nine
# node cross-region cluster if more AZs per region are desired.
cat <<- EOF > "rg1-az1-x.conf"
server_name: rg1-az1-x
server_tags: [rg:1, az:1]
port: 4231
http_port: 8225
include cross-region-shared.conf
cluster: {
  name: xr
  port: 6231
  routes: [
    nats-route://127.0.0.1:6231,
    nats-route://127.0.0.1:6232,
    nats-route://127.0.0.1:6233,
  ]
}
gateway {
  name: xr
  port: 7231
  include gateway-routes.conf
}
jetstream: {
  store_dir: /tmp/nats/storage/rg1-az1-x
}
EOF

cat <<- EOF > "rg2-az2-x.conf"
server_name: rg2-az2-x
server_tags: [rg:2, az:2]
port: 4232
include cross-region-shared.conf
cluster: {
  name: xr
  port: 6232
  routes: [
    nats-route://127.0.0.1:6231,
    nats-route://127.0.0.1:6232,
    nats-route://127.0.0.1:6233,
  ]
}
gateway {
  name: xr
  port: 7232
  include gateway-routes.conf
}
jetstream: {
  store_dir: /tmp/nats/storage/rg2-az2-x
}
EOF

cat <<- EOF > "rg3-az3-x.conf"
server_name: rg3-az3-x
server_tags: [rg:3, az:3]
port: 4233
include cross-region-shared.conf
cluster: {
  name: xr
  port: 6233
  routes: [
    nats-route://127.0.0.1:6231,
    nats-route://127.0.0.1:6232,
    nats-route://127.0.0.1:6233,
  ]
}
gateway {
  name: xr
  port: 7233
  include gateway-routes.conf
}
jetstream: {
  store_dir: /tmp/nats/storage/rg3-az3-x
}
EOF

# Start a server for each configuration and sleep a second in
# between so the seeds can startup and get healthy.
for c in $(ls rg*.conf); do
  echo "Starting server ${c%.*}"
  nats-server -c $c -l "${c%.*}.log" > /dev/null 2>&1 &
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

echo 'Cluster 3 healthy?'
curl --fail --silent \
  --retry 5 \
  --retry-delay 1 \
  http://localhost:8224/healthz; echo

echo 'Cluster 4 healthy?'
curl --fail --silent \
  --retry 5 \
  --retry-delay 1 \
  http://localhost:8225/healthz; echo

# Show the server lit and JetStream report.
nats --user sys --password sys server info rg2-az1
nats --user sys --password sys server list
nats --user sys --password sys server report jetstream

# Create a cross-region stream specifying the `xr` cluster.
nats --user user --password user stream add \
  --cluster=xr \
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
  --subjects="events.*" \
  EVENTS

# Create a regional stream specifying one of the regional clusters, e.g.
# `rg2`.
nats --user user --password user stream add \
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

# Report out the streams.
nats --user user --password user stream report
