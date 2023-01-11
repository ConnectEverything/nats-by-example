#!/bin/sh

set -euo pipefail

unset NATS_URL

# Define a accounts.config included in each of the individual
# node configs.
#
# Notice the `mappings` option defined on the `APP` account
# which takes a published message such as `events.1` and
# will map it to `events.1.4`, where the last token indicate
# the deterministic partition number. In this case, the second
# token `1` is mapped to a partition between 0 and 4 (five total
# partitions).
#
# *Note: you can have more than one token when defining the
# partitioning.*
cat <<- EOF > accounts.conf
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

# Declare the configuration per node including the shared
# configuration. Note, if decentralized auth is being used
# mappings can be defined via the `nsc` tool on account
# JWTs.
cat <<- EOF > n1.conf
port: 4222
http_port: 8222
server_name: n1

include accounts.conf

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

include accounts.conf

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

include accounts.conf

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


# Start the cluster and wait a few seconds to choose the leader.
nats-server -c n1.conf > /dev/null 2>&1 &
nats-server -c n2.conf > /dev/null 2>&1 &
nats-server -c n3.conf > /dev/null 2>&1 &

sleep 3

# Save and select the default context for convenience.
nats context save \
  --server "nats://localhost:4222,nats://localhost:4223,nats://localhost:4224" \
  --user app \
  --password app \
  default > /dev/null

nats context select default > /dev/null

# Create five streams modeling partitions.
# Note the `--subjects` option correponds to the subject mapping
# we defined above, `events.*.0`, `events.*.1`, etc.
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

# Report the stream to confirm they are present.
nats stream report

# Run a benchmark of one publisher using synchronous acks.
# The `--multisubject` option the sequence number as a token
# to the base subject `events` in this case. The `--stream`
# option is simply an override to using the default stream
# but does not impact behavior.
nats bench \
  --js \
  --multisubject \
  --pub 1 \
  --msgs 200000 \
  --syncpub \
  --no-progress \
  --stream "events-0" \
  "events"

# As expected the throughput is quite low due to the synchronous
# publish.
# However, looking at the stream report again, we see the messages
# have been distributed to each stream.
nats stream report

# Let's purge the streams and try the benchmark again with
# a async batch of 100.
for i in $(seq 0 4); do
  nats stream purge -f "events-$i" > /dev/null
done

nats bench \
  --js \
  --multisubject \
  --pub 1 \
  --pubbatch 100 \
  --msgs 200000 \
  --no-progress \
  --stream "events-0" \
  "events"

# As expected the throughput increases.
nats stream report

# Purging one more time, we will also increase the number of
# concurrent publishers to 5.
for i in $(seq 0 4); do
  nats stream purge -f "events-$i" > /dev/null
done

nats bench \
  --js \
  --multisubject \
  --pub 5 \
  --pubbatch 100 \
  --msgs 200000 \
  --no-progress \
  --stream "events-0" \
  "events"

# The throughput increases a bit more. Do note, that throughput
# and latency will highly depend on the quality of the network,
# and the resources available on the servers hosing the streams.
#
# *Note: If you are looking at the output on NATS by Example and are
# unimpressed with the numbers, do know this example ran in a
# GitHub Actions runner which fairly low-resourced having 2 cpu
# cores and 7GB of memory. Given this examples sets up three
# servers with five streams all of which are replicated as well
# as running the CLI for benchmarking (which takes up resources),
# true performance will suffer in comparison to a production setup.*
nats stream report
