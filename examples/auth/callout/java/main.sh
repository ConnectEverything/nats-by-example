#!/bin/bash

set -eou pipefail

NATS_URL="nats://localhost:4222"

# Use a static issuer Nkey
# Typically this would be generated using the `nsc generate nkey --account`
# or programmatically with one of the `nkeys` libraries.
ISSUER_NKEY="ABJHLOVMPA4CI6R5KLNGOB4GSLNIY7IOUPAJC4YFNDLQVIOBYQGUWVLA"
ISSUER_NSEED="SAANDLKMXL6CUS3CP52WIXBEDN6YJ545GDKC65U5JZPPV6WH6ESWUA6YAI"

# Create multi-account config with auth callout enabled. Encryption is not enabled in this example.
cat <<- EOF > server.conf
accounts {
  AUTH {
    users: [
      { user: auth, password: auth }
    ]
  }
  APP {}
  SYS {}
}

authorization {
  auth_callout {
    issuer: $ISSUER_NKEY
    users: [ auth ]
    account: AUTH
  }
}

system_account: SYS
EOF

# Users emulating a user directory backend.
# For documentation purposes only,
# the java example has a hardcoded copy of this.
cat <<- EOF > users.json
{
  "sys": {
    "pass": "sys",
    "account": "SYS"
  },
  "alice": {
    "pass": "alice",
    "account": "APP"
  },
  "bob": {
    "pass": "bob",
    "account": "APP",
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


# Start the server.
nats-server -c server.conf > /dev/null 2>&1 &
sleep 1

# Start the auth callout service providing the auth credentials
# to connect and the issuer seed.
java -cp /app/example.jar example.Main &

sleep 2

# Users sys, alice and bob are known to the example code
# Confirm alice can connect and publish.
echo 'Test alice'
nats --user alice --password alice pub test 'hello from alice'

# Confirm bob cannot publish to a subject they are not allowed to.
echo 'Test bob on denied subject'
nats --user bob --password bob pub test 'hello from bob' || true

# Confirm bob can connect and publish to a subject they are allowed to.
echo 'Test bob on allowed subject'
nats --user bob --password bob pub bob.test 'hello from bob'

# Confirm an unknown user cannot connect.
echo 'Test unknown user'
nats --user pam --password pam pub test 'hello from pam' || true

# Confirm the system account user has the expected system permission.
echo 'Test system account user'
nats --user sys --password sys server list
