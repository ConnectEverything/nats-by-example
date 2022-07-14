#!/bin/bash

set -eo pipefail

targets=(
  auth/create-jwts
)

for target in ${targets[@]}; do
  echo "\$ go run ./examples/$target/main.go" > examples/$target/output.txt
  go run ./examples/$target/main.go >> examples/$target/output.txt
done
