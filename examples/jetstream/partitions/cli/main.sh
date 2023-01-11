#!/bin/sh

set -euo pipefail

unset NATS_URL

# Define the system account to be included by all configurations.
cat <<- EOF > shared.conf
accounts: {
  SYS: {
    users: [{user: sys, password: sys}]
  }
  APP: {
    jetstream: true,
    users: [{user: app, password: app}]
    mappings: {
      "events.*": "events.{{wildcard(1)}}.{{partition(5,1)}}"
    }
  }
}

system_account: SYS
EOF

# Create the *east* and *west* server configurations.
# A requirement of JetStream is to have a cluster block defined
# since this is how available storage resources are determined
# for placement of streams and consumers.
#
# In a production deployment, at least three nodes per cluster
# are recommended which supports creating R3 streams. In this
# test setup, only R1 streams can be created since streams replicas
# do not cross gateway connections.
cat <<- EOF > n1.conf
port: 4222
http_port: 8222
server_name: n1

include shared.conf

jetstream: {
  store_dir: "./n1"
}

cluster: {
  name: c1,
  port: 6222,
  routes: [
    "nats-route://0.0.0.0:6222",
    "nats-route://0.0.0.0:6223",
    "nats-route://0.0.0.0:6224",
  ],
}
EOF

cat <<- EOF > n2.conf
port: 4223
http_port: 8223
server_name: n2

include shared.conf

jetstream: {
  store_dir: "./n2"
}

cluster: {
  name: c1,
  port: 6223,
  routes: [
    "nats-route://0.0.0.0:6222",
    "nats-route://0.0.0.0:6223",
    "nats-route://0.0.0.0:6224",
  ],
}
EOF

cat <<- EOF > n3.conf
port: 4224
http_port: 8224
server_name: n3

include shared.conf

jetstream: {
  store_dir: "./n3"
}

cluster: {
  name: c1,
  port: 6224,
  routes: [
    "nats-route://0.0.0.0:6222",
    "nats-route://0.0.0.0:6223",
    "nats-route://0.0.0.0:6224",
  ],
}
EOF


# Start the servers and sleep for a few seconds to startup.
nats-server -c n1.conf &
N1_PID=$?

nats-server -c n2.conf  &
N2_PID=$?

nats-server -c n3.conf  &
N3_PID=$?

sleep 3


# Save and select the default context to use all seed servers.
nats context save \
  --server "nats://localhost:4222,nats://localhost:4223,nats://localhost:4224" \
  --user app \
  --password app \
  default

nats context select default

# Create five streams modeling partitions.
for i in $(seq 0 4); do
  nats stream add \
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
    --subjects="events.*.$i" \
    "events-$i" > /dev/null
done

nats stream report

nats bench \
  --js \
  --multisubject \
  --pub 1 \
  --msgs 500000 \
  --syncpub \
  --no-progress \
  --stream "events-0" \
  "events"

nats stream report

for i in $(seq 0 4); do
  nats stream purge -f "events-$i" > /dev/null
done

nats bench \
  --js \
  --multisubject \
  --pub 1 \
  --pubbatch 100 \
  --msgs 500000 \
  --no-progress \
  --stream "events-0" \
  "events"

nats stream report
