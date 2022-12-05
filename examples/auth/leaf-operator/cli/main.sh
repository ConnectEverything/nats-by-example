#!/bin/bash

set -euo pipefail

unset NATS_URL

# ### Bootstrap the operator

# Create the operator, generate a signing key (which is a best practice),
# and initialize the default SYS account and sys user.
nsc add operator --generate-signing-key --sys --name local

# A follow-up edit of the operator enforces signing keys are used for
# accounts as well. Setting the server URL is a convenience so that
# it does not need to be specified with call `nsc push`.
nsc edit operator --require-signing-keys \
  --account-jwt-server-url "nats://127.0.0.1:4222"

# This command generates the bit of configuration to be used by the server
# to setup the embedded JWT resolver.
nsc generate config --nats-resolver --sys-account SYS > resolver.conf

# ### Server configs for supercluster
#
# This emulates a supercluster consisting of three clusters with
# three nodes each.

cat <<- EOF > hub-shared.conf
jetstream: {
  domain: hub
}
include resolver.conf
EOF

cat <<- EOF > gateway-routes.conf
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
  {name: central, urls: [
    nats://localhost:7228,
    nats://localhost:7229,
    nats://localhost:7230,
  ]},
]
EOF

# Define the server configs for east cluster.
cat <<- EOF > "east-az1.conf"
server_name: east-az1
port: 4222
http_port: 8222
include hub-shared.conf
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
  include gateway-routes.conf
}
leafnodes {
  port: 7422
}
EOF

cat <<- EOF > "east-az2.conf"
server_name: east-az2
port: 4223
include hub-shared.conf
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
  include gateway-routes.conf
}
leafnodes {
  port: 7423
}
EOF

cat <<- EOF > "east-az3.conf"
server_name: east-az3
port: 4224
include hub-shared.conf
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
  include gateway-routes.conf
}
leafnodes {
  port: 7424
}
EOF

# Server configs for west cluster.
cat <<- EOF > "west-az1.conf"
server_name: west-az1
port: 4225
http_port: 8223
include hub-shared.conf
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
  include gateway-routes.conf
}
leafnodes {
  port: 7425
}
EOF

cat <<- EOF > "west-az2.conf"
server_name: west-az2
port: 4226
include hub-shared.conf
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
  include gateway-routes.conf
}
leafnodes {
  port: 7426
}
EOF

cat <<- EOF > "west-az3.conf"
server_name: west-az3
port: 4227
include hub-shared.conf
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
  include gateway-routes.conf
}
leafnodes {
  port: 7427
}
EOF

# Server configs for central cluster.
cat <<- EOF > "central-az1.conf"
server_name: central-az1
port: 4228
http_port: 8224
include hub-shared.conf
cluster: {
  name: central
  port: 6228
  routes: [
    nats-route://127.0.0.1:6228,
    nats-route://127.0.0.1:6229,
    nats-route://127.0.0.1:6230,
  ]
}
gateway {
  name: central
  port: 7228
  include gateway-routes.conf
}
leafnodes {
  port: 7428
}
EOF

cat <<- EOF > "central-az2.conf"
server_name: central-az2
port: 4229
include hub-shared.conf
cluster: {
  name: central
  port: 6229
  routes: [
    nats-route://127.0.0.1:6228,
    nats-route://127.0.0.1:6229,
    nats-route://127.0.0.1:6230,
  ]
}
gateway {
  name: central
  port: 7229
  include gateway-routes.conf
}
leafnodes {
  port: 7429
}
EOF

cat <<- EOF > "central-az3.conf"
server_name: central-az3
port: 4230
include hub-shared.conf
cluster: {
  name: central
  port: 6230
  routes: [
    nats-route://127.0.0.1:6228,
    nats-route://127.0.0.1:6229,
    nats-route://127.0.0.1:6230,
  ]
}
gateway {
  name: central
  port: 7230
  include gateway-routes.conf
}
leafnodes {
  port: 7430
}
EOF

# ### Bring up the supercluster

# Start a server for each configuration and sleep a second in
# between so the seeds can startup and get healthy.
for c in $(ls *-az*.conf); do
  echo "Starting.. $c"
  nats-server -c $c > /dev/null 2>&1 &
  sleep 1
done

# Wait until the servers up and healthy.
echo 'East cluster healthy?'
curl --fail --silent \
  --retry 5 \
  --retry-delay 1 \
  http://localhost:8222/healthz; echo

echo 'West cluster healthy?'
curl --fail --silent \
  --retry 5 \
  --retry-delay 1 \
  http://localhost:8223/healthz; echo

echo 'Central cluster healthy?'
curl --fail --silent \
  --retry 5 \
  --retry-delay 1 \
  http://localhost:8224/healthz; echo

# ### Setup APP account
#
# Next we need to create an account intended for application usage. It is
# currently a two-step process to create the account, followed by
# generating the signing key and setting JetStream limits (default is disabled).
nsc add account APP
nsc edit account APP \
  --sk generate \
  --js-disk-storage -1 \
  --js-mem-storage -1

# Push to the cluster.
nsc push -a APP

# ### Setup single user

nsc add user --account APP app

# ### Try different leaf configurations

# This leaf config shows how the default global account on the leaf
# can be mapped to the hub's system account. The effect is that a
# client connecting to the leaf node without any auth can perform
# system account operations.
cat <<- EOF > "leaf.conf"
server_name: leaf
port: 4231

leafnodes {
  remotes: [
    {
      urls: [
        "nats-leaf://127.0.0.1:7422"
        "nats-leaf://127.0.0.1:7423"
        "nats-leaf://127.0.0.1:7424"
      ]
      credentials: "/nsc/nkeys/creds/local/SYS/sys.creds"
      account: '\$G'
    }
  ]
}
EOF

nats-server -c leaf.conf & #> /dev/null 2>&1 &
LEAF_PID=$!

# Setup and use a leaf context for convenience.
nats context save leaf \
  --server "nats://localhost:4231"

nats context select leaf

# These commands work since the requests to the `$SYS.>` subjects
# transparently forward to the hub. Note that this leaf node does
# not even have JetStream enabled...
nats server list
nats server report jetstream

# We will alter the config to establish two separate connections,
# one mapping to the remote APP account and one to the remote SYS.
# What this means in practice is that a local user connected to the
# leaf which is associated with an account mapped to a remote, will
# implicitly 
cat <<- EOF > "leaf.conf"
server_name: leaf
port: 4231

leafnodes {
  remotes: [
    {
      urls: [
        "nats-leaf://127.0.0.1:7425"
        "nats-leaf://127.0.0.1:7426"
        "nats-leaf://127.0.0.1:7427"
      ]
      credentials: "/nsc/nkeys/creds/local/APP/app.creds"
      account: "APP"
    },
    {
      urls: [
        "nats-leaf://127.0.0.1:7425"
        "nats-leaf://127.0.0.1:7426"
        "nats-leaf://127.0.0.1:7427"
      ]
      credentials: "/nsc/nkeys/creds/local/SYS/sys.creds"
      account: "\$SYS"
    }
  ]
}

accounts {
  \$SYS: {
    users: [{user: sys, password: sys}]
  }
  APP: {
    users: [{user: app, password: app}]
  }
}

no_auth_user: app
EOF

# Restart the leaf since not all options are able to be reloaded.
nats-server --signal stop=$LEAF_PID
nats-server -c leaf.conf & #> /dev/null 2>&1 &
LEAF_PID=$!

sleep 1

# We can create and list streams on the hub even though the leaf
# still does not have JetStream enabled. This is because the local
# `APP` account maps to the hub `APP` account that can create JetStream
# assets in the hub.
nats stream add events \
  --js-domain=hub \
  --subjects="events.*" \
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
  --no-deny-purge

sleep 0.5

# Run a local responder for demonstration..
nats reply 'events.*' 'local events' &
REPLY_PID=$!

# Note, these requests will be received by both the local responder
# (the `nats reply` above) as well as the traverse the leaf
# connection to be received by the stream. The responder will be
# the local service merely because it is closer and will be the first
# to respond.
nats req 'events.1'
nats req 'events.2'

# Listing the streams, we will notice the stream has two messages.
nats --js-domain=hub stream list

# Using the local `sys` user, we can still get the server report.
nats --user sys --password sys server report jetstream

# Stop the background responder.
kill $REPLY_PID

SYS_ID=$(nsc describe account SYS --json | jq -r .sub)
APP_ID=$(nsc describe account APP --json | jq -r .sub)
SYS_JWT=$(nsc describe account SYS -R)
APP_JWT=$(nsc describe account APP -R)

echo "system account: $SYS_ID"
echo "app account: $APP_ID"

# Next we will include change the leaf node to use the same operator
# and the `memory` resolver with the preloaded `SYS` and `APP`
# accounts. This setup _could_ be useful to leverage the same
# trust chain, however put more constraints on the credentials used
# for the remote compared to the credentials used by the clients
# connecting to the leaf.
cat <<- EOF > "leaf.conf"
server_name: leaf
port: 4231

operator: /nsc/nats/nsc/stores/local/local.jwt

resolver_preload {
  $SYS_ID: $SYS_JWT
  $APP_ID: $APP_JWT
}

resolver: memory

leafnodes {
  remotes: [
    {
      urls: [
        "nats-leaf://127.0.0.1:7425"
        "nats-leaf://127.0.0.1:7426"
        "nats-leaf://127.0.0.1:7427"
      ]
      credentials: "/nsc/nkeys/creds/local/APP/app.creds"
      account: $APP_ID
    },
    {
      urls: [
        "nats-leaf://127.0.0.1:7425"
        "nats-leaf://127.0.0.1:7426"
        "nats-leaf://127.0.0.1:7427"
      ]
      credentials: "/nsc/nkeys/creds/local/SYS/sys.creds"
      account: $SYS_ID
    }
  ]
}
EOF

# Restart the leaf.
nats-server --signal stop=$LEAF_PID
nats-server -c leaf.conf & #> /dev/null 2>&1 &
LEAF_PID=$!

sleep 1

# Listing the streams, we will notice the stream has two messages.
nats --creds /nsc/nkeys/creds/local/APP/app.creds --js-domain=hub stream list

# Using the local `sys` user, we can still get the server report.
nats --creds /nsc/nkeys/creds/local/SYS/sys.creds server report jetstream

# Next, we will use the cache resolver which allows for getting updates
# from a full resolver (a node in the cluster). We still need to preload
# the accounts and JWTs required for the remote connections.
cat <<- EOF > "leaf.conf"
server_name: leaf
port: 4231

operator: /nsc/nats/nsc/stores/local/local.jwt

resolver {
  type: cache
  dir: "./leaf-jwts"
}

resolver_preload {
  $SYS_ID: $SYS_JWT
  $APP_ID: $APP_JWT
}

leafnodes {
  remotes: [
    {
      urls: [
        "nats-leaf://127.0.0.1:7425"
        "nats-leaf://127.0.0.1:7426"
        "nats-leaf://127.0.0.1:7427"
      ]
      credentials: "/nsc/nkeys/creds/local/APP/app.creds"
      account: $APP_ID
    },
    {
      urls: [
        "nats-leaf://127.0.0.1:7425"
        "nats-leaf://127.0.0.1:7426"
        "nats-leaf://127.0.0.1:7427"
      ]
      credentials: "/nsc/nkeys/creds/local/SYS/sys.creds"
      account: $SYS_ID
    }
  ]
}
EOF

# Restart the leaf.
nats-server --signal stop=$LEAF_PID
nats-server -c leaf.conf & #> /dev/null 2>&1 &
LEAF_PID=$!

sleep 1

# Listing the streams, we will notice the stream has two messages.
nats --creds /nsc/nkeys/creds/local/APP/app.creds --js-domain=hub stream list

# Using the local `sys` user, we can still get the server report.
nats --creds /nsc/nkeys/creds/local/SYS/sys.creds server report jetstream

# If we were to edit the `APP` account and push, we will see the leaf
# is listed in one of the servers the new JWT is pushed to.
nsc edit account APP --payload 1024
nsc push -a APP
