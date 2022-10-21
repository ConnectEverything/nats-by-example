#!/bin/sh

set -euo pipefail

# Define the system account to be included by all configurations.
cat <<- EOF > auth.conf
jetstream: {}

authorization: {
  users: [
    {
      user: admin, password: "admin",
      permissions: {
        subscribe: ">",
        publish: ">"
      }
    },
    {
      user: router001, password: "router001",
      permissions: {
        subscribe: [
          "_INBOX_router001.>"
        ],
        publish: [
          "\$JS.API.STREAM.INFO.KV_test_kv",
          "\$KV.test_kv.dev.001.>",
          "\$JS.API.DIRECT.GET.KV_test_kv.\$KV.test_kv.dev.001.>"
        ]
      }
    },
  ]
}
EOF

# Start the servers and sleep to startup.
nats-server -c auth.conf > /dev/null 2>&1 &

sleep 1

# Save a context for each user.
nats context save \
  --user admin \
  --password admin \
  admin

nats context save \
  --user router001 \
  --password router001 \
  --inbox-prefix _INBOX_router001 \
  router001

# Add a KV "dev001" with admin user.
nats --context=admin kv add \
  --history=1 \
  --ttl=0 \
  --replicas=1 \
  --max-value-size=-1 \
  --max-bucket-size=-1 \
  --storage=file \
  test_kv

nats --context=router001 \
  kv put test_kv 'dev.001.temp' '80c'

nats --context=router001 \
  kv get test_kv 'dev.001.temp'
