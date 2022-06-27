#!/bin/bash

#set -euo pipefail

for f in $(ls pids); do
  nats-server --signal stop="pids/$f" 2> /dev/null
done