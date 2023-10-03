#!/bin/bash

set -eou pipefail

NATS_URL="nats://localhost:4222"

# Helper function to copy the signing key.
function extract_signing_key() {
  sk=$(nsc describe account $1 --field 'nats.signing_keys[0]' | tr -d '"')
  cat "/root/.local/share/nats/nsc/keys/keys/${sk:0:1}/${sk:1:2}/${sk}.nk"
}

# ### Bootstrap the resolver
#
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

cat resolver.conf

# Create the most basic config that simply includes the generated
# resolver config.
cat <<- EOF > server.conf
include resolver.conf
EOF

# Start the server.
nats-server -c server.conf > /dev/null 2>&1 &
sleep 1

# ### Setup application accounts
#
# Setup two application accounts for demonstration.
nsc add account APP1
nsc edit account APP1 --sk generate

nsc add account APP2
nsc edit account APP2 --sk generate

# Push the two app accounts up to the server.
nsc push -A

# Create a user per account.
nsc add user --account APP1 --name app1
nsc add user --account APP2 --name app2

# Generate creds for the two app accounts to show that they work as expected
# without auth callout enabled.
nsc generate creds --account APP1 --name app1 > app1.creds
nsc generate creds --account APP2 --name app2 > app2.creds

nats --creds app1.creds pub test 'hello from app1'
nats --creds app2.creds pub test 'hello from app2'

# ### Setup auth account and configure callout
#
# Create an `AUTH` account which will be registered as the
# account for the auth callout service.
nsc add account AUTH
nsc edit account AUTH --sk generate

# Create a user for the auth callout service. Extract the public key
# of the user so that it can be used when configuring auth callout on
# the account.
nsc add user --account AUTH --name auth
USER_PUB=$(nsc describe user --account AUTH --name auth --field sub | jq -r)

# Generate an Xkey for auth callout.
nsc generate nkey --curve 2> auth.xk
XKEY_SEED=$(sed -n "1,1p" auth.xk)
XKEY_KEY=$(sed -n "2,1p" auth.xk)

APP1_PUB=$(nsc describe account APP1 --field sub | jq -r)
APP2_PUB=$(nsc describe account APP2 --field sub | jq -r)

# Edit the AUTH account to allow it to be used by the auth callout service.
# The `--allowed-account` option is used to define which accounts this
# account is allowed to bind authorized users to. In this case, `*` indicates
# that any account can be bound. However if there are select accounts, they
# would be listed via their public nkey.
nsc edit authcallout \
  --account AUTH \
  --curve $XKEY_KEY \
  --auth-user $USER_PUB \
  --allowed-account '*'

# Push the AUTH account up to the server.
nsc push -A

sleep 2

# Confirm existing creds still work even with auth callout enabled.
nats --creds app1.creds pub test 'hello from app1'
nats --creds app2.creds pub test 'hello from app2'

# ### Setup auth callout service
#
# Next, we need the signing keys for the application accounts that the
# auth callout service is *allowed* to create and bind users to.
# First we extract the signing key for each account.
extract_signing_key APP1 > APP1.nk
extract_signing_key APP2 > APP2.nk

# We also need the signing key of the AUTH account itself to sign
# the responses.
extract_signing_key AUTH > AUTH.nk

# In order for the auth callout service to be able to connect, we need
# the credentials for the `auth` user.
nsc generate creds --account AUTH --name auth > auth.creds

# Write out a couple users emulating a user directory backend.
cat <<- EOF > users.json
{
  "alice": {
    "pass": "alice",
    "account": "APP1"
  },
  "bob": {
    "pass": "bob",
    "account": "APP2",
    "permissions": {
      "pub": {
        "allow": ["bob.>"]
      },
      "sub": {
        "allow": ["bob.>"]
      },
      "resp": {
        "max": 1
      }
    }
  }
}
EOF

# Start the auth callout service passing the creds, account signing keys,
# as well as the Xkey seed that was generated earlier.
echo 'Starting auth callout service...'
service \
  -nats.creds=auth.creds \
  -issuer.seed=AUTH.nk \
  -xkey.seed=$XKEY_SEED \
  -signing.keys=$APP1_PUB:APP1.nk,$APP2_PUB:APP2.nk \
  -users=users.json &

sleep 2

# The final requirement for clients to be able to connect is having
# a set of credentials of the AUTH acount which will be used to by
# the server to delegate to the correct auth callout service.
# Add a sentinel user for the AUTH account that is required
# to be passed along with additional credentials.
nsc add user --account AUTH --name sentinel --deny-pubsub ">"
nsc generate creds --account AUTH --name sentinel > sentinel.creds

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
