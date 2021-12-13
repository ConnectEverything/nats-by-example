#!/bin/bash

set -euo pipefail

nats -s localhost:4222,localhost:4223,localhost:4224 consumer add \
  --deliver "all" \
  --target "foo-c" \
  --deliver-group "foo-c" \
  --ack "none" \
  --replay "instant" \
  --filter "" \
  --heartbeat "0s" \
  --no-flow-control \
  --no-headers-only \
  "foo" "foo-c"

nats -s localhost:4222,localhost:4223,localhost:4224 consumer add \
  --deliver "all" \
  --target "bar-c" \
  --deliver-group "bar-c" \
  --ack "none" \
  --replay "instant" \
  --filter "" \
  --heartbeat "0s" \
  --no-flow-control \
  --no-headers-only \
  "bar" "bar-c"