#!/bin/sh

set -euo pipefail

# Define configuration for two nodes.
cat <<- EOF > n1.conf
port: 4222
http_port: 8222
server_name: n1

jetstream: {}

cluster: {
  name: c1,
  port: 6222,
  routes: [
    "nats-route://0.0.0.0:6222"
    "nats-route://0.0.0.0:6223"
  ],
}
EOF

cat <<- EOF > n2.conf
port: 4223
http_port: 8223
server_name: n2

jetstream: {}

cluster: {
  name: c1,
  port: 6223,
  routes: [
    "nats-route://0.0.0.0:6222"
    "nats-route://0.0.0.0:6223"
  ],
}
EOF


# Start the servers and sleep for a few seconds to startup.
nats-server -c n1.conf > /dev/null 2>&1 &
nats-server -c n2.conf > /dev/null 2>&1 &

sleep 3

# Wait until the servers are healthy.
curl --fail --silent \
  --retry 10 \
  --retry-delay 1 \
  http://localhost:8222/healthz > /dev/null

# Create the two streams with their own respective subjects.
nats kv add n1 --history=10
nats kv add n2 --history=10

# Edit the streams to source from each other, filtered to the
# stream's bounded subject.
nats stream edit --force --config n1-edit.json KV_n1
nats stream edit --force --config n2-edit.json KV_n2

# Publish a message to each stream.
nats kv put n1 foo
nats kv put n2 bar

# Both streams should have two messages (in different orders).
echo 'List n1:'
nats kv list n1
nats stream view KV_n1

echo 'List n2:'
nats kv list n2
nats stream view KV_n2

nats stream report
nats stream report
