#!/bin/sh

set -euo pipefail

# For this example, we are going to have a service connected
# to the main server and then another client send a request
# via a connection to the leaf node.
NATS_MAIN_URL="nats://0.0.0.0:4222"
NATS_LEAF_URL="nats://0.0.0.0:4223"

# The `nats` CLI provides a way to manage different _contexts_ by name.
# We save two, one for the main server and one for the leaf node.
# In subsequent calls to `nats`, we can pass the `--context` option
# to refer to either server.
nats context save main \
  --server "$NATS_MAIN_URL" \

nats context save leaf \
  --server "$NATS_LEAF_URL"

# Create the most basic server config which enables leaf node connections.
echo 'Creating the main server conf...'
cat <<- EOF > main.conf
port: 4222
leafnodes: {
  port: 7422
}
EOF

# The second config is for the leaf node itself. This needs to define
# the leaf node _remotes_ which is the main server it will be connecting
# to.
echo 'Creating the leaf node conf...'
cat <<- EOF > leaf.conf
port: 4223
leafnodes: {
  remotes: [
    {url: "nats-leaf://0.0.0.0:7422"}
  ]
}
EOF

# Start the main server first.
nats-server -c main.conf 2> /dev/null &
MAIN_PID=$!

sleep 1

# Now we can start the leaf node which uses the credentials for the
# remote connection.
nats-server -c leaf.conf 2> /dev/null &
LEAF_PID=$!

sleep 1

# Here we create a simple service which is connected via the main
# server. We will put this in the background since this will block
# while serving.
nats --context main reply 'greet' 'hello from main' &
SERVICE_PID=$!

# Tiny sleep to ensure the service is connected.
sleep 1

# Let's try out a request that is performed by a client connecting
# to the leaf node. As expected, this will get routed to the service
# connected to the main server.
nats --context leaf request 'greet' ''

# Let's start another service connected to the leaf node servicing
# the same subject.
nats --context leaf reply 'greet' 'hello from leaf' &
SERVICE2_PID=$!

# NATS is smart enough to route messages to _closest_ service relative
# to where the client request comes from.
nats --context leaf request 'greet' ''
nats --context leaf request 'greet' ''
nats --context leaf request 'greet' ''

# If we remove the leaf-base service, it will fallback to the main
# service.
kill $SERVICE2_PID
nats --context leaf request 'greet' ''

# Finally stop the service and servers.
kill $SERVICE_PID
kill $LEAF_PID
kill $MAIN_PID
