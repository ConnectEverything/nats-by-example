#!/bin/sh

set -xuo pipefail

# First, we will create an empty configuration file
# and startup a server in the background.
touch server.conf

nats-server -c server.conf -l log.txt & 
SERVER_PID=$!

sleep 0.5

# If we try to run a command requests server info, we will get back
# an error indicating we need "system privileges" and "appropriate
# permissions" 🤔.
nats server info

# NATS has this default _system account_ named `$SYS` which is dedicated
# for server operations and monitoring. To activate we need to define
# a user associated with this account. We can do this by updating our
# config file with the accounts block.
cat <<- EOF > server.conf
accounts: {
  \$SYS: {
    users: [{user: sys, password: pass}]
  }
}
EOF

# Simply reload the server with the `--signal` option.
nats-server --signal reload=$SERVER_PID


# For convenience, and so we don't type the password on the command line,
# we can save a new `sys` context.
nats context save sys \
  --user sys \
  --password pass


# Now we can try getting the server info again. We need to set the
# user and password.
nats --context sys server info


kill $SERVER_PID
