#!/bin/bash

set -eo pipefail

for path in $(find examples/ -path *.go); do
  gofmt -w $path
done
