#!/bin/bash

set -eo pipefail

./tools/vet.sh
./tools/format.sh
./tools/measure.sh
./tools/outputs.sh
./tools/generate.sh

