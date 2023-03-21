#!/bin/sh

NATS_URL="nats://0.0.0.0:4222"

nsc add operator --generate-signing-key --sys --name local

nsc edit operator --require-signing-keys \
  --account-jwt-server-url "$NATS_URL"

nsc generate config --nats-resolver --sys-account SYS > resolver.conf

# Start the server.
nats-server -c resolver.conf 2> /dev/null &

sleep 1

# Print the creds before a fresh import..
cat /nsc/nkeys/creds/local/SYS/sys.creds

# Import (introducing the newline) and generate new creds.
nsc describe user -a SYS -n sys --raw > sys.jwt
nsc import user --overwrite -f sys.jwt

# Print the creds after the import.. they remain without
# the newline.
cat /nsc/nkeys/creds/local/SYS/sys.creds

# Generating the creds (with newline) to use for the client.
nsc generate creds -a SYS -n sys > sys.creds

# Authorization error...
nats --creds sys.creds server list

# Try with original on disk (without a newline).
nats --creds /nsc/nkeys/creds/local/SYS/sys.creds server list
