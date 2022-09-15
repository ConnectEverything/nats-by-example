#!/bin/bash

set -euo pipefail

# Create a shared configuration which enables JetStream and defines the
# `unique_tag` option which enforces all replicas for a given stream or
# consumer to be placed on nodes with different availability zones (AZ).
cat <<- EOF > accounts-shared.conf
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

cat <<- EOF > rg-arbiter.conf
server_name: arbiter
port: 4228
http_port: 8224
include accounts-shared.conf

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

cat <<- EOF > gateway-routes.conf
gateways: [
  {name: arbiter, urls: [
    nats://localhost:7228,
  ]},
]
EOF

cat <<- EOF > jetstream-shared.conf
jetstream: {
  unique_tag: "az:"
}
include accounts-shared.conf
EOF

# Define the server configs for region 1 cluster.
cat <<- EOF > "rg1-az1.conf"
server_name: rg1-az1
server_tags: [az:1]
port: 4222
http_port: 8222
include jetstream-shared.conf
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
EOF

cat <<- EOF > "rg1-az2.conf"
server_name: rg1-az2
server_tags: [az:2]
port: 4223
include jetstream-shared.conf
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
EOF

cat <<- EOF > "rg1-az3.conf"
server_name: rg1-az3
server_tags: [az:3]
port: 4224
include jetstream-shared.conf
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
EOF

# Server configs for region 2 cluster.
cat <<- EOF > "rg2-az1.conf"
server_name: rg2-az1
server_tags: [az:1]
port: 4225
http_port: 8223
include jetstream-shared.conf
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
EOF

cat <<- EOF > "rg2-az2.conf"
server_name: rg2-az2
server_tags: [az:2]
port: 4226
include jetstream-shared.conf
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
EOF

cat <<- EOF > "rg2-az3.conf"
server_name: rg2-az3
server_tags: [az:3]
port: 4227
include jetstream-shared.conf
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

echo 'Arbiter healthy?'
curl --fail --silent \
  --retry 5 \
  --retry-delay 1 \
  http://localhost:8224/healthz; echo

# Show the server list and JetStream report.
nats --user sys --password sys server info rg2-az1
nats --user sys --password sys server list
nats --user sys --password sys server report jetstream

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
