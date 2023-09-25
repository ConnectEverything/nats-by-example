#!/bin/bash

set -eou pipefail

NATS_URL="nats://localhost:4222"

# Create the operator, generate a signing key (which is a best practice),
# and initialize the default SYS account and sys user.
nsc add operator --generate-signing-key --sys --name local

# A follow-up edit of the operator enforces signing keys are used for
# accounts as well. Setting the server URL is a convenience so that
# it does not need to be specified with call `nsc push`.
nsc edit operator \
  --require-signing-keys \
  --account-jwt-server-url "$NATS_URL"

# This command generates the bit of configuration to be used by the server
# to setup the embedded JWT resolver.
nsc generate config \
  --nats-resolver \
  --sys-account SYS > resolver.conf

# Create the most basic config that simply include the generated
# resolver config.
cat <<- EOF > server.conf
include resolver.conf
EOF

# Start the server.
nats-server -c server.conf -D & #> /dev/null 2>&1 &
SERVER_PID=$!

sleep 1

# Create an `AUTH` account which will be registered as the
# account for the auth callout service.
nsc add account AUTH
nsc edit account AUTH --sk generate

AUTH_SK=$(nsc describe account AUTH --field 'nats.signing_keys[0]' | tr -d '"')
cp "/root/.local/share/nats/nsc/keys/keys/${AUTH_SK:0:1}/${AUTH_SK:1:2}/${AUTH_SK}.nk" auth.nk

# Create a user for the auth callout service. Extract the public key
# of the user so that it can be used when configuring the auth callout.
nsc add user --account AUTH --name auth
USER_PUB=$(nsc describe user --account AUTH --name auth --field sub)

# Edit the AUTH account to allow it to be used by the auth callout service.
# The `--allowed-account` option is used to define which accounts this
# account is allowed to bind authorized users to. In this case, `*` indicates
# that any account can be bound. However if there are select accounts, they
# would be listed via their public nkey.
nsc edit authcallout \
  --account AUTH \
  --auth-user $USER_PUB \
  --allowed-account '*'

# Create two application accounts to demonstrate how the auth callout
# service can be configured to designate users for different accounts.
nsc add account APP1
nsc edit account APP1 --sk generate

nsc add account APP2
nsc edit account APP2 --sk generate

# Push all accounts up to the server. This will make the server aware of
# AUTH service which will resort in the server delegating authentication.
nsc push -A

# ### Creds and keys for the service
#
# In order for the auth callout service to be able to connect, we need
# the credentials for the `auth` user.
nsc generate creds --account AUTH --name auth > auth.creds

# Next, we need the signing keys for the application accounts that the
# auth callout service is *allowed* to create and bind users to.
# First we extract the signing key for each account and then find the
# private nkey within the nsc data directory.
APP1_SK=$(nsc describe account APP1 --field 'nats.signing_keys[0]' | tr -d '"')
APP2_SK=$(nsc describe account APP2 --field 'nats.signing_keys[0]' | tr -d '"')

echo 'Copied signing keys...'
cp "/root/.local/share/nats/nsc/keys/keys/${APP1_SK:0:1}/${APP1_SK:1:2}/${APP1_SK}.nk" app1.nk
cp "/root/.local/share/nats/nsc/keys/keys/${APP2_SK:0:1}/${APP2_SK:1:2}/${APP2_SK}.nk" app2.nk

# Start the auth callout service passing the creds and account signing keys.
echo 'Starting auth callout service...'
service \
  -creds=auth.creds \
  -account=auth.nk \
  -targets=app1.nk,app2.nk \
  -users=directory.json \
  &

sleep 2

# Add a sentinel user for the AUTH account that is required
# to be passed along with additional credentials.
nsc add user \
  --account AUTH \
  --name sentinel \
  --deny-pubsub ">"

nsc generate creds \
  --account AUTH \
  --name sentinel > sentinel.creds

echo 'Client request from alice...'
client \
  -creds=sentinel.creds \
  -user alice \
  -pass alice


echo 'Client request from bob...'
client \
  -creds=sentinel.creds \
  -user bob \
  -pass bob


echo 'Client request from zach...'
client \
  -creds=sentinel.creds \
  -user zach \
  -pass zach

