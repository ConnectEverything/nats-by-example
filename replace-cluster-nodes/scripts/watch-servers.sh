#!/bin/bash

set -euo pipefail

watch nats -s localhost:4222,localhost:4223,localhost:4224 --user sys --password pass server list