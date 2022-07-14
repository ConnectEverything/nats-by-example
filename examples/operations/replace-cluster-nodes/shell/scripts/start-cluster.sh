#!/bin/bash

set -euo pipefail


rm -rf {pids,logs,data}
mkdir -p {pids,logs,data}

nats-server --config configs/n0.conf &
nats-server --config configs/n1.conf &
nats-server --config configs/n2.conf &
