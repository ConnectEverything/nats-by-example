#!/bin/sh

set -euo pipefail

# Create the *east* and *west* node configurations.
cat <<- EOF > east.conf
port: 4222
server_name: n1

accounts: {
  SYS: {
    users: [{user: sys, password: pass}]
  }
}

system_account: SYS

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
    {name: "west", urls: ["nats://0.0.0.0:7223"]},
    {name: "south", urls: ["nats://0.0.0.0:7224"]},
  ]
}

EOF

cat <<- EOF > west.conf
port: 4223
server_name: n2

accounts: {
  SYS: {
    users: [{user: sys, password: pass}]
  }
}

system_account: SYS

jetstream: {}

cluster: {
  name: west,
  port: 6223,
  routes: [
    "nats-route://0.0.0.0:6223"
  ],
}

gateway: {
  name: "west",
  port: 7223,
  gateways: [
    {name: "east", urls: ["nats://0.0.0.0:7222"]},
    {name: "west", urls: ["nats://0.0.0.0:7223"]},
    {name: "south", urls: ["nats://0.0.0.0:7224"]},
  ]
}
EOF

cat <<- EOF > south.conf
port: 4224
server_name: n3

accounts: {
  SYS: {
    users: [{user: sys, password: pass}]
  }
}

system_account: SYS

jetstream: {}

cluster: {
  name: south,
  port: 6224,
  routes: [
    "nats-route://0.0.0.0:6224"
  ],
}

gateway: {
  name: "south",
  port: 7224,
  gateways: [
    {name: "east", urls: ["nats://0.0.0.0:7222"]},
    {name: "west", urls: ["nats://0.0.0.0:7223"]},
    {name: "south", urls: ["nats://0.0.0.0:7224"]},
  ]
}
EOF


# Start the two servers, each with a prefix to the log lines to distinguish
# them while running in the background.
nats-server -c east.conf -D 2>&1 | sed 's|^|[east] |' &
EAST_PID=$!

nats-server -c west.conf -D 2>&1 | sed 's|^|[west] |' &
WEST_PID=$!

nats-server -c south.conf -D 2>&1 | sed 's|^|[south] |' &
SOUTH_PID=$!

sleep 10

nats server list \
  --server "nats://localhost:4222" \
  --user sys \
  --password pass
