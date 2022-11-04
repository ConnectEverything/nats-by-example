#!/bin/sh

set -euo pipefail

# Define the system account to be included by all configurations.
cat <<- EOF > sys.conf
accounts: {
  SYS: {
    users: [{user: sys, password: sys}]
  }
  APP: {
    jetstream: true,
    users: [{user: app, password: app}]
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

include sys.conf

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

include sys.conf

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

include sys.conf

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
N1_PID=$!

nats-server -c n2.conf &
N2_PID=$!

nats-server -c n3.conf &
N3_PID=$!

sleep 3

# Save and select the default context to use all seed servers.
nats context save \
  --server "nats://localhost:4222,nats://localhost:4223,nats://localhost:4224" \
  --user app \
  --password app \
  app 

nats context select app


# Show the server list which will indicate the clusters and
# gateway connections as well as the JetStream server report.
nats --user sys --password sys server list
nats --user sys --password sys server report jetstream

# Add a stream.
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
  --subjects="orders" \
  ORDERS

# Publish some messages.
nats req orders --count=100 'Message {{Count}}'

# Force a step down of the leader.
nats stream cluster step-down ORDERS

# Report the new leader.
nats --user sys --password sys server report jetstream

# Publish more messages.
nats req orders --count=100 'Message {{Count}}'

nats stream list

# Now determine the leader and kill the server.
LEADER=$(nats stream info ORDERS --json | jq -r '.cluster.leader')
echo "$LEADER is the leader"

case $LEADER in
  "n1")
    kill $N1_PID
    ;;
  "n2")
    kill $N2_PID
    ;;
  "n3")
    kill $N3_PID
    ;;
esac

# Publish more messages.
nats req orders --count=100 'Message {{Count}}'

nats stream list

nats --user sys --password sys server report jetstream
