#!/bin/sh

set -uo pipefail

NATS_URL="${NATS_URL:-nats://localhost:4222}"

# Create the operator, generate a signing key (which is a best practice),
# and initialize the default SYS account and sys user.
nsc add operator --generate-signing-key --sys --name local

# A follow-up edit of the operator enforces signing keys are used for
# accounts as well. Setting the server URL is a convenience so that
# it does not need to be specified with call `nsc push`.
nsc edit operator --require-signing-keys \
  --account-jwt-server-url "$NATS_URL"

# Next we need to create an account intended for application usage. It is
# currently a two-step process to create the account, followed by
# generating the signing key.
nsc add account APP
nsc edit account APP --sk generate

# This command generates the bit of configuration to be used by the server
# to setup the embedded JWT resolver.
nsc generate config --nats-resolver --sys-account SYS > resolver.conf

# Create the most basic config that simply include the generated
# resolver config.
cat <<- EOF > server.conf
include resolver.conf
EOF

# Start the server.
nats-server -c server.conf 2> /dev/null &
SERVER_PID=$!

sleep 1

# Push the account up to the server.
nsc push -a APP

# The next two users emulate consumers of the service. They can publish
# on their own prefixed subject as well as publish to services scoped the
# their respective name.
nsc add user --account APP joe \
  --allow-pub 'joe' \
  --allow-sub 'joe' \

nsc add user --account APP pam \
  --allow-pub 'pam' \
  --allow-sub 'pam'

# A nice side effect of this is that now, joe and pam can't subscribe
# to each other's subjects, however, what about `_INBOX.>`? Let's observe
# the current behavior and then see how we can address this.

# First, let's save a few contexts for easier reference.
nats context save joe \
  --nsc nsc://local/APP/joe

nats context save pam \
  --nsc nsc://local/APP/pam

# Attempt to subscribe to the other user.. should get permission violation.
echo 'Attempting to subscribe to pam by joe..'
nats --context joe sub 'pam'
echo 'Attempting to subscribe to joe by pam..'
nats --context pam sub 'joe'

# Subscribe to the correct user..
nats --context joe sub 'joe' &
nats --context pam sub 'pam' &

# Publish to the wrong user..
nats --context joe pub 'pam' ''
nats --context pam pub 'sue' ''

# Publish to the right user..
nats --context joe pub 'joe' ''
nats --context pam pub 'pam' ''
