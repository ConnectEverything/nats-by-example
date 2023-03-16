#!/bin/sh

set -euo pipefail

# For this example, we are going to have a service connected
# to the main server and then another client send a request
# via a connection to the leaf node.
NATS_MAIN_URL="nats://0.0.0.0:4222"
NATS_LEAF_URL="nats://0.0.0.0:4223"

# Create the operator, generate a signing key (which is a best practice),
# and initialize the default SYS account and sys user.
nsc add operator --generate-signing-key --sys --name local

# A follow-up edit of the operator enforces signing keys are used for
# accounts as well. Setting the server URL is a convenience so that
# it does not need to be specified with call `nsc push`.
nsc edit operator --require-signing-keys \
  --account-jwt-server-url "$NATS_MAIN_URL"

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
  --server "$NATS_MAIN_URL" \
  --nsc nsc://local/APP/user 

nats context save main-sys \
  --server "$NATS_MAIN_URL" \
  --nsc nsc://local/SYS/sys

nats context save leaf-user \
  --server "$NATS_LEAF_URL"

# This command generates the bit of configuration to be used by the server
# to setup the embedded JWT resolver.
nsc generate config --nats-resolver --sys-account SYS > resolver.conf

# Create the most basic server config which enables leaf node connections
# and include the JWT resolver config.
echo 'Creating the main server conf...'
cat <<- EOF > main.conf
port: 4222
leafnodes: {
  port: 7422
}

include resolver.conf
EOF

# The second config is for the leaf node itself. This needs to define
# the leaf node _remotes_ which is the main server it will be connecting
# to.
echo 'Creating the leaf node conf...'
cat <<- EOF > leaf.conf
port: 4223
leafnodes: {
  remotes: [
    {
      url: "nats-leaf://0.0.0.0:7422",
      credentials: "/root/.local/share/nats/nsc/keys/creds/local/APP/user.creds"
    }
  ]
}
EOF

# Start the main server first.
nats-server -c main.conf 2> /dev/null &
MAIN_PID=$!

sleep 1

# We need to put up the `APP` account JWT we created previously to the
# main server so that the user credentials file used both for the
# client providing the service and the client making the request is
# trusted.
echo 'Pushing the account JWT...'
nsc push -a APP

# Now we can start the leaf node which uses the credentials for the
# remote connection.
nats-server -c leaf.conf 2> /dev/null &
LEAF_PID=$!

sleep 1

# Connecting directly to the main server with the user creds, we can
# create a simple service that will reply to any request published
# to `greet` with the text `hello`. This is put in the background
# since this will block while serving.
nats --context main-user reply 'greet' 'hello' &
SERVICE_PID=$!

# Tiny sleep to ensure the service is connected.
sleep 1

# For this CLI invocation, we connect to the _leaf_ server and send
# a request to `greet`. Notice two things, one is that no credentials
# file is specified since the leaf server does not have authentication
# setup. Instead the `leafnodes.remotes` section of the config defines
# the main server and provides the credentials so that the leaf node
# is authenticated for forwarding messaging. This request will be
# transparently fullfilled by the service connected to the main server.
nats --context leaf-user request 'greet' ''

# Finally stop the service and servers.
kill $SERVICE_PID
kill $LEAF_PID
kill $MAIN_PID
