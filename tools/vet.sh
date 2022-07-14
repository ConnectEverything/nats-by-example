#!/bin/bash

set -eo pipefail

go vet ./examples/...
