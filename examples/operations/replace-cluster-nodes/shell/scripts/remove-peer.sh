#!/bin/bash

set -euo pipefail

nats -s localhost:4222,localhost:4223,localhost:4224 \
  --user sys \
  --password pass \
  server raft peer-remove --force $1