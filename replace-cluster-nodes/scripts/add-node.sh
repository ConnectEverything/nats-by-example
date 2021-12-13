#!/bin/bash

set -euo pipefail

rm -rf data/$1 logs/$1 pids/$1
nats-server --config "configs/$1.conf" &