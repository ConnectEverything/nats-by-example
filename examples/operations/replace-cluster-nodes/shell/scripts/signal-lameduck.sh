#!/bin/bash

set -euo pipefail

nats-server --signal ldm="pids/$1"