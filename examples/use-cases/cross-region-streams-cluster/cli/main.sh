#!/bin/bash

set -euo pipefail

NATS_URL="nats://localhost:4222"

# Create a shared configuration which enables JetStream and defines the
# `unique_tag` option which enforces all replicas for a given stream or
# consumer to be placed on nodes with different availability zones (AZ).
cat <<- EOF > shared.conf
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

# Create the shared clustered config defining the seed routes.
cat <<- EOF > cluster.conf
name: c1
routes: [
  nats-route://127.0.0.1:6222,
  nats-route://127.0.0.1:6223,
  nats-route://127.0.0.1:6224,
] 
EOF

# Define nine server configurations modeling a cluster with three nodes
# in each region across three AZs. NATS does not currently support
# declaring tags with logical OR (or exclusive OR), so valid combinations
# can be precomputed as tags and then used when creating streams. In this
# case, the `xr:` tag encodes the cluster/AZ combination while still adhering
# to the `unique_tag` requirement.
# Each index corresponds to a region (e.g us-east) and the value at the
# index corresponds to an AZ, e.g. us-east4.
# Thus a tag `xr:231` can be read as a cross-region stream where one replica
# is placed in region 1 in AZ 2, another in region 2 in AZ 3, and the third
# replica in region 3 in AZ 1.
# In addition to the `xr:` tags, the standard `rg:` (region) and `az:` (AZ)
# tags are set to support _regional_ streams which is demonstrated below. 
cat <<- EOF > "rg1-az1.conf"
server_name: rg1-az1
server_tags: [rg:1, az:1, xr:123, xr:132]
port: 4222
http_port: 8222
include shared.conf
cluster: {
  port: 6222
  include cluster.conf
}
jetstream: {
  store_dir: /tmp/nats/storage/rg1-az1
}
EOF

cat <<- EOF > "rg1-az2.conf"
server_name: rg1-az2
server_tags: [rg:1, az:2, xr:213, xr:231]
port: 4223
include shared.conf
cluster: {
  port: 6223
  include cluster.conf
}
jetstream: {
  store_dir: /tmp/nats/storage/rg1-az2
}
EOF

cat <<- EOF > "rg1-az3.conf"
server_name: rg1-az3
server_tags: [rg:1, az:3, xr:312, xr:321]
port: 4224
include shared.conf
cluster: {
  port: 6224
  include cluster.conf
}
jetstream: {
  store_dir: /tmp/nats/storage/rg1-az3
}
EOF

cat <<- EOF > "rg2-az1.conf"
server_name: rg2-az1
server_tags: [rg:2, az:1, xr:213, xr:312]
port: 4225
include shared.conf
cluster: {
  port: 6225
  include cluster.conf
}
jetstream: {
  store_dir: /tmp/nats/storage/rg2-az1
}
EOF

cat <<- EOF > "rg2-az2.conf"
server_name: rg2-az2
server_tags: [rg:2, az:2, xr:123, xr:321]
port: 4226
include shared.conf
cluster: {
  port: 6226
  include cluster.conf
}
jetstream: {
  store_dir: /tmp/nats/storage/rg2-az2
}
EOF

cat <<- EOF > "rg2-az3.conf"
server_name: rg2-az3
server_tags: [rg:2, az:3, xr:132, xr:231]
port: 4227
include shared.conf
cluster: {
  port: 6227
  include cluster.conf
}
jetstream: {
  store_dir: /tmp/nats/storage/rg2-az3
}
EOF

cat <<- EOF > "rg3-az1.conf"
server_name: rg3-az1
server_tags: [rg:3, az:1, xr:231, xr:321]
port: 4228
include shared.conf
cluster: {
  port: 6228
  include cluster.conf
}
jetstream: {
  store_dir: /tmp/nats/storage/rg3-az1
}
EOF

cat <<- EOF > "rg3-az2.conf"
server_name: rg3-az2
server_tags: [rg:3, az:2, xr:132, xr:312]
port: 4229
include shared.conf
cluster: {
  port: 6229
  include cluster.conf
}
jetstream: {
  store_dir: /tmp/nats/storage/rg3-az2
}
EOF

cat <<- EOF > "rg3-az3.conf"
server_name: rg3-az3
server_tags: [rg:3, az:3, xr:123, xr:213]
port: 4230
include shared.conf
cluster: {
  port: 6230
  include cluster.conf
}
jetstream: {
  store_dir: /tmp/nats/storage/rg3-az3
}
EOF

# Start a server for each configuration and sleep a second in
# between so the seeds can startup and get healthy.
for c in $(ls rg*.conf); do
  echo "Starting server ${c%.*}"
  nats-server -c $c > /dev/null 2>&1 &
  sleep 1
done

# Wait until the servers up and healthy.
echo 'Healthy?'
curl --fail --silent \
  --retry 5 \
  --retry-delay 1 \
  http://localhost:8222/healthz; echo

# Show the server lit and JetStream report.
nats --user sys --password sys server list
nats --user sys --password sys server report jetstream

# Create a cross-region stream using one of the pre-defined `xr:` tags.
nats --user user --password user stream add \
  --tag=xr:123 \
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

# Create a regional stream which relies on the `unique_tag` to ensure
# each replicas in a different AZ. This will create a stream in region 2
# due to the `rg:2` tag
nats --user user --password user stream add \
  --tag=rg:2 \
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
