#!/bin/sh

set -uo pipefail

NATS_URL="${NATS_URL:-nats://localhost:4222}"

# For this example, we will create three users. The first one called
# `greeter` will be used for the `greet` service. It can subscribe to a
# dedicated subject in order to service requests.
# The other two users emulate consumers of the service. They can publish
# on their own prefixed subject as well as publish to services scoped the
# their respective name. The `_INBOX.>` subscription permission is
# required in order to receive replies from a requester.
# A nice side effect of this is that now, joe and pam can't subscribe
# to each other's subjects (since an explicit allow has been declared),
# however, what about `_INBOX.>`? Let's observe
# the current behavior and then see how we can address this.
cat <<- EOF > server.conf
accounts: {
  APP: {
    users: [
      {
        user: greeter,
        password: greeter,
        permissions: {
          sub: {
            allow: ["services.greet"]
          },
          # Enables this service to reply to the requestor's inbox.
          allow_responses: true
        }
      },
      {
        user: joe,
        password: joe,
        permissions: {
          pub: {
            allow: ["joe.>", "services.*"]
          },
          sub: {
            allow: ["_INBOX.>"]
          },
        }
      },
      {
        user: pam,
        password: pam,
        permissions: {
          pub: {
            allow: ["pam.>", "services.*"]
          },
          sub: {
            allow: ["_INBOX.>"]
          },
        }
      },
    ]
  }
}
EOF

# Start the server with the config.
nats-server -c server.conf 2> /dev/null &
SERVER_PID=$!

# Save a few contexts for convenience...
nats context save greeter \
  --user greeter --password greeter

nats context save joe \
  --user joe --password joe

nats context save pam \
  --user pam --password pam

# Then we startup the greeter service that simply returns a unique reply ID.
nats --context greeter \
  reply 'services.greet' \
  'Reply {{ID}}' &

GREETER_PID=$!

# Tiny sleep to ensure the service is connected.
sleep 0.5

# Send a greet request from joe and pam.
nats --context joe request 'services.greet' ''
nats --context pam request 'services.greet' ''

# But can pam also receive replies from requests sent by joe? Indeed,
# by subscribing to the inbox.
nats --context pam sub '_INBOX.>' &
INBOX_SUB_PID=$!

# When joe sends a request, the reply will come back to him, but also
# be received by pam. 🤨 This is actually expected and generally fine
# since _accounts_ are expected to be the isolation boundary, at a
# certain level of scale, creating users with granular permissions
# becomes increasingly necessary.
nats --context joe request 'services.greet' ''

# Sinces inboxes are randomly generated by the server, by default we
# can't pin down the specific set of subjects to provide permission to.
# However, as a client, there is the option of defining an explicit
# *inbox prefix* other than `_INBOX`.
nats --context joe --inbox-prefix _INBOX_joe request 'services.greet' ''

# Now that we can have a differentiated inbox prefix, we set the only
# allow to be the one specific to the user.
cat <<- EOF > server.conf
accounts: {
  APP: {
    users: [
      {
        user: greeter,
        password: greeter,
        permissions: {
          sub: {
            allow: ["services.greet"]
          },
          # Enables this service to reply to the requestor's inbox.
          allow_responses: true
        }
      },
      {
        user: joe,
        password: joe,
        permissions: {
          pub: {
            allow: ["joe.>", "services.*"]
          },
          sub: {
            allow: ["_INBOX_joe.>"]
          },
        }
      },
      {
        user: pam,
        password: pam,
        permissions: {
          pub: {
            allow: ["pam.>", "services.*"]
          },
          sub: {
            allow: ["_INBOX_pam.>"]
          },
        }
      },
    ]
  }
}
EOF

# Reload the server to pick up the new config.
echo 'Reloading the server with new config...'
nats-server --signal reload=$SERVER_PID

# Stop the previous service to pick up the new permission. Now pam
# cannot subscribe to the general `_INBOX` nor joe's specific one
# since it does not have an explicit allow.
kill $INBOX_SUB_PID
nats --context pam sub '_INBOX.>'
nats --context pam sub '_INBOX_joe.>'

# Now we can send requests and receive replies in isolation.
nats --context joe --inbox-prefix _INBOX_joe request 'services.greet' ''
nats --context pam --inbox-prefix _INBOX_pam request 'services.greet' ''

# Finally stop the service and server.
kill $GREETER_PID
kill $SERVER_PID
