#!/bin/bash

set -euo pipefail

# ## Save connection contexts
# The NATS CLI can be used to save and manage *contexts* which
# declare the server(s) to connect to as well as authentication 
# and TLS options. For this example, we will create two contexts
# for the NATS and STAN servers. However, if channels and/or
# subscriptions need to be migrated to different accounts, then
# multiple contexts can be used.
echo 'Saving contexts...'
nats context save stan \
  --server $STAN_URL

nats context save nats \
  --server $NATS_URL

# ## Generate STAN data
# To showcase the migration, three channels and subscriptions will be
# used. The first channel, `foo`, has a `max_msgs` limit of 400 and
# one subscription. The second channel, `bar`, has a `max_msgs`
# limit of 500 and two subscriptions, one of which is a queue.
# The third channel, `baz`, has no limits and no subscriptions.
# 1000 messages are published to all channels.
echo 'Generating STAN data...'
generate-stan-data \
  --context stan \
  --cluster $STAN_CLUSTER \
  --client app

# ## Define the migration config
# Define a config file required by [`stan2js`][1] to declare
# which channels and subscriptions are to be migrated, as well
# as some associated stream and consumer options. These options
# are not exhaustive since many options on streams and consumers
# can be changed after the migration. Refer to the `stan2js`
# README for the full set of options.
# [1]: https://github.com/nats-io/stan2js
echo 'Creating config file...'
cat <<-EOF > config.yaml
stan: stan
nats: nats
cluster: test-cluster
client: stan2js

channels:
  foo:
    stream:
      name: FOO
      replicas: 1

  bar:
    stream:
      name: BAR
      max_consumers: 10
      max_bytes: 1GB

  baz:
    stream:
      name: BAZ
      max_age: 1h

clients:
  app:
    context: stan
    subscriptions:
      sub-foo:
        channel: foo
        consumer:
          name: foo
          pull: true
      
      sub-bar-q:
        channel: bar
        queue: q
        consumer:
          name: bar-queue
          queue: bar-queue

      sub-bar:
        channel: bar
        consumer:
          name: bar-consumer
EOF

# ## Run the migration!
echo 'Running migration...'
stan2js config.yaml

# ## Verify the migration
# Since the NATS CLI supports introspecting JetStream assets,
# we can use it to verify that the migration was successful.
# Also we can inspect the streams to confirm the headers and
# data look correct.
echo 'Report the streams...'
nats --context nats stream report

echo 'View 3 messages from FOO...'
nats --context nats stream view FOO 3

echo 'View 3 messages from BAR...'
nats --context nats stream view BAR 3

echo 'View 3 messages from BAZ...'
nats --context nats stream view BAZ 3

echo 'Report the consumers...'
nats --context nats consumer report FOO
nats --context nats consumer report BAR
nats --context nats consumer report BAZ

