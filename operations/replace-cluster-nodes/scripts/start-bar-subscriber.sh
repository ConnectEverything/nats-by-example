#!/bin/bash

set -euo pipefail

nats -s localhost:4222,localhost:4223,localhost:4224 sub \
  --queue "bar-c" \
  --ack \
  "bar-c"