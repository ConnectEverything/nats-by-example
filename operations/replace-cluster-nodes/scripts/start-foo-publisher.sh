#!/bin/bash

set -euo pipefail

nats -s localhost:4222,localhost:4223,localhost:4224 pub --timeout 5s --sleep 1500ms --count 10000 -w "foo.test" "Hello {{Count}}"