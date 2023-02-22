#!/bin/sh

set -euo pipefail

# Define configuration for two nodes.
cat <<- EOF > n1.conf
port: 4222
http_port: 8222
server_name: n1

mappings: {
  "\$KV.mykv.>" : "\$KV.mykv.n1.>"
}

jetstream: {
  domain: n1
  store_dir: "./n1"
}

leafnodes: {
  port: 7422
  remotes = [
    {
      url: "nats-leaf://localhost:7423"
    }
  ]
}
EOF

cat <<- EOF > n2.conf
port: 4223
http_port: 8223
server_name: n2

mappings: {
  "\$KV.mykv.>" : "\$KV.mykv.n2.>"
}

jetstream: {
  domain: n2
  store_dir: "./n2"
}

leafnodes: {
  port: 7423
  remotes = [
    {
      url: "nats-leaf://localhost:7422"
    }
  ]
}
EOF


# Start the servers and sleep for a few seconds to startup.
nats-server -c n1.conf > /dev/null 2>&1 &
N1_PID=$!

nats-server -c n2.conf > /dev/null 2>&1 &
N2_PID=$!

sleep 3

# Wait until the servers are healthy.
curl --fail --silent \
  --retry 10 \
  --retry-delay 1 \
  http://localhost:8222/healthz > /dev/null

curl --fail --silent \
  --retry 10 \
  --retry-delay 1 \
  http://localhost:8223/healthz > /dev/null

# Save two contexts, one for each leaf.
nats context save n1 \
  --server nats://localhost:4222

nats context save n2 \
  --server nats://localhost:4223

# Create a KV on each leaf.
nats --context=n1 kv add mykv --history=10
nats --context=n2 kv add mykv --history=10

# Edit to source from one another with a subject transform.
# e.g. $KV.mykv.> -> $KV.mykv.> and vice versa for the other.
nats --context=n1 stream edit --force --config n1-edit.json KV_mykv
nats --context=n2 stream edit --force --config n2-edit.json KV_mykv

# Put one value each.
nats --context=n1 kv put mykv foo ''
nats --context=n2 kv put mykv foo ''

echo 'Messages in n1 KV...'
nats --context=n1 stream view KV_mykv

echo 'Messages in n2 KV...'
nats --context=n2 stream view KV_mykv

# Report the streams. Each should have two messages.
nats --context=n1 stream report
nats --context=n2 stream report

# Take n1 down.
nats-server --signal=quit=$N1_PID

# Put two more keys in n2.
nats --context=n2 kv put mykv baz ''
nats --context=n2 kv put mykv qux ''

# Report on n2, which should not have four messages.
nats --context=n2 stream report
nats --context=n2 stream view KV_mykv

# Turn n1 back on.
nats-server -c n1.conf & #> /dev/null 2>&1 &
N1_PID=$!

sleep 2

# Wait for healthy.
curl --fail --silent \
  --retry 10 \
  --retry-delay 1 \
  http://localhost:8222/healthz > /dev/null

# Let n1 catch up with messages from n2
nats --context=n1 stream report
nats --context=n1 stream view KV_mykv
