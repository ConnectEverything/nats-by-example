#!/bin/bash

set -euo pipefail

unset NATS_URL

cat <<- EOF > hub-shared.conf
accounts: {
  SYS: {
    users: [{user: sys, pass: sys}]
  }
  APP: {
    jetstream: true
    users: [{user: app, pass: app}]
  }
}

system_account: SYS
EOF

# Define the server configs for cluster.
cat <<- EOF > "n1.conf"
server_name: n1
port: 4222
http_port: 8222
include hub-shared.conf
jetstream {
  domain: hub
  store_dir: "./n1"
}
cluster: {
  name: hub 
  port: 6222
  routes: [
    nats-route://127.0.0.1:6222,
    nats-route://127.0.0.1:6223,
    nats-route://127.0.0.1:6224,
  ]
}
leafnodes {
  port: 7422
}
EOF

cat <<- EOF > "n2.conf"
server_name: n2
port: 4223
include hub-shared.conf
jetstream {
  domain: hub
  store_dir: "./n2"
}
cluster: {
  name: hub
  port: 6223
  routes: [
    nats-route://127.0.0.1:6222,
    nats-route://127.0.0.1:6223,
    nats-route://127.0.0.1:6224,
  ]
}
leafnodes {
  port: 7423
}
EOF

cat <<- EOF > "n3.conf"
server_name: n3
port: 4224
include hub-shared.conf
jetstream {
  domain: hub
  store_dir: "./n3"
}
cluster: {
  name: hub
  port: 6224
  routes: [
    nats-route://127.0.0.1:6222,
    nats-route://127.0.0.1:6223,
    nats-route://127.0.0.1:6224,
  ]
}
leafnodes {
  port: 7424
}
EOF

# ### Bring up the cluster

# Start a server for each configuration and sleep a second in
# between so the seeds can startup and get healthy.
echo "Starting n1..."
nats-server -c n1.conf -P n1.pid > /dev/null 2>&1 &
sleep 1

echo "Starting n2..."
nats-server -c n2.conf -P n2.pid > /dev/null 2>&1 &
sleep 1

echo "Starting n3..."
nats-server -c n3.conf -P n3.pid > /dev/null 2>&1 &
sleep 1

# Wait until the servers up and healthy.
echo 'Hub cluster healthy?'
curl --fail --silent \
  --retry 5 \
  --retry-delay 1 \
  http://localhost:8222/healthz; echo

cat <<- EOF > leaf-shared.conf
accounts: {
  SYS: {
    users: [{user: sys, pass: sys}]
  }
  APP: {
    jetstream: true
    users: [{user: app, pass: app}]
  }
}

system_account: SYS

leafnodes {
  remotes [
    {
      urls: [
        nats-leaf://app:app@localhost:7422,
        nats-leaf://app:app@localhost:7423,
        nats-leaf://app:app@localhost:7424,
      ]
      account: APP
    }
  ]
}
EOF


# Define the server configs for cluster.
cat <<- EOF > "l1.conf"
server_name: l1
port: 4225
http_port: 8223
include leaf-shared.conf
jetstream {
  domain: leaf
  store_dir: "./l1"
}
cluster: {
  name: leaf
  port: 6225
  routes: [
    nats-route://127.0.0.1:6225,
    nats-route://127.0.0.1:6226,
    nats-route://127.0.0.1:6227,
  ]
}
EOF

cat <<- EOF > "l2.conf"
server_name: l2
port: 4226
include leaf-shared.conf
jetstream {
  domain: leaf
  store_dir: "./l2"
}
cluster: {
  name: leaf
  port: 6226
  routes: [
    nats-route://127.0.0.1:6225,
    nats-route://127.0.0.1:6226,
    nats-route://127.0.0.1:6227,
  ]
}
EOF

cat <<- EOF > "l3.conf"
server_name: l3
port: 4227
include leaf-shared.conf
jetstream {
  domain: leaf
  store_dir: "./l3"
}
cluster: {
  name: leaf
  port: 6227
  routes: [
    nats-route://127.0.0.1:6225,
    nats-route://127.0.0.1:6226,
    nats-route://127.0.0.1:6227,
  ]
}
EOF

# ### Bring up the leaf cluster

echo "Starting l1..."
nats-server -c l1.conf -P l1.pid > /dev/null 2>&1 &
sleep 1

echo "Starting l2..."
nats-server -c l2.conf -P l2.pid > /dev/null 2>&1 &
sleep 1

echo "Starting l3..."
nats-server -c l3.conf -P l3.pid > /dev/null 2>&1 &
sleep 1

# Wait until the servers up and healthy.
echo 'Leaf cluster healthy?'
curl --fail --silent \
  --retry 5 \
  --retry-delay 1 \
  http://localhost:8223/healthz; echo

nats context save --user app --password app -s nats://localhost:4222 hub-app

nats context save --user app --password app -s nats://localhost:4225 leaf-app


nats context save --user sys --password sys -s nats://localhost:4222 hub-sys

nats context save --user sys --password sys -s nats://localhost:4225 leaf-sys

cat <<- EOF > origin.json
{
  "name": "events",
  "subjects": [
    "events.*"
  ],
  "retention": "limits",
  "max_consumers": -1,
  "max_msgs_per_subject": -1,
  "max_msgs": -1,
  "max_bytes": -1,
  "max_age": 0,
  "max_msg_size": -1,
  "storage": "file",
  "discard": "old",
  "num_replicas": 2,
  "duplicate_window": 120000000000,
  "sealed": false,
  "deny_delete": false,
  "deny_purge": false,
  "allow_rollup_hdrs": false,
  "allow_direct": false,
  "mirror_direct": false
}
EOF

cat <<- EOF > mirror.json
{
  "name": "events",
  "retention": "limits",
  "max_consumers": -1,
  "max_msgs_per_subject": -1,
  "max_msgs": -1,
  "max_bytes": -1,
  "max_age": 0,
  "max_msg_size": -1,
  "storage": "file",
  "discard": "old",
  "num_replicas": 2,
  "mirror": {
    "name": "events",
    "external": {
      "api": "\$JS.hub.API"
    }
  },
  "sealed": false,
  "deny_delete": false,
  "deny_purge": false,
  "allow_rollup_hdrs": false,
  "allow_direct": false,
  "mirror_direct": false
}
EOF

nats --context hub-app stream add --config origin.json
nats --context leaf-app stream add --config mirror.json

nats --context hub-app stream report
nats --context leaf-app stream report

echo 'Starting bench...'
nats bench \
  --context hub-app \
  --pub 1 \
  --js \
  --size 10k \
  --stream events \
  --msgs 10000 \
  --pubbatch 100 \
  --no-progress \
  --multisubject \
  events

sleep 1

# Report the streams
nats --context hub-app stream report
nats --context leaf-app stream report

