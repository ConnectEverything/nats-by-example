#!/bin/sh

set -xeuo pipefail

NATS_URL="nats://0.0.0.0:4222"

# Create the operator, generate a signing key (which is a best practice),
# and initialize the default SYS account and sys user.
nsc add operator --generate-signing-key --sys --name local

# A follow-up edit of the operator enforces signing keys are used for
# accounts as well. Setting the server URL is a convenience so that
# it does not need to be specified with call `nsc push`.
nsc edit operator --require-signing-keys \
  --account-jwt-server-url "$NATS_URL"

# Next we need to create an account intended for application usage. The
# `SYS` account should be used for operational purposes. These commands create
# the `APP` account, generates a signing key, and then creates a user named
# `user`.
nsc add account APP
nsc edit account APP --sk generate
nsc add user --account APP user

# Check out the current settings of nsc.
nsc env

# The `nats` CLI provides a way to manage different _contexts_ by name.
# Here we define the server and the credentials (via `nsc` integration)
# (notice the operator/account/user hierarchy).
# We save two one for the main server and one for the leaf node. Note
# how didn't provide credentials for the leaf node..
nats context save main-user \
  --nsc nsc://local/APP/user 

nats context save main-sys \
  --nsc nsc://local/SYS/sys

# This command generates the bit of configuration to be used by the server
# to setup the embedded JWT resolver.
nsc generate config --nats-resolver --sys-account SYS > resolver.conf

# Define the system account to be included by all configurations.
cat <<- EOF > sys.conf
include resolver.conf
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

jetstream: {}

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

jetstream: {}

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

jetstream: {}

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
nats-server -c n2.conf  &
nats-server -c n3.conf  &

sleep 3

nats context save user \
  --nsc nsc://local/APP/user

nats context select user


nsc add mapping --from "foo" --to "bar" --weight "50"
nsc add mapping --from "foo" --to "baz" --weight "50"

nsc push

nats reply bar 'from bar' &
nats reply baz 'from baz' &

sleep 0.5

nats req --count 10 foo
