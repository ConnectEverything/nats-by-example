#!/bin/bash

set -euo pipefail

unset NATS_URL

cat <<- EOF > a_1.conf
cluster: {
  name: hengli
  port: 15333
  routes: [nats-route://127.0.0.1:15333,nats-route://127.0.0.1:15334,nats-route://127.0.0.1:15335]
}
http_port: 18222
jetstream: {
  domain: hengli
  store_dir: "./jetstream/hengli_1"
}
leafnodes: {
  port: 14333
  remotes: [{
    account: hengli
    urls: [nats-leaf://_hengliConnUser_:_hengliConnUser_@127.0.0.1:24333,nats-leaf://_hengliConnUser_:_hengliConnUser_@127.0.0.1:24334,nats-leaf://_hengliConnUser_:_hengliConnUser_@127.0.0.1:24335]
  }]
}
port: 14222
server_name: hengli_1

accounts: {
  hengli: {
    jetstream: enabled
    users:[
      {user: _hengliConnUser_, password: _hengliConnUser_}
      {user: hengli, password: hengli, permissions: {
        publish: {
          allow:["hengli.to.wsw.test","\$JS.API.>"]
        },
        subscribe: {
          allow:["\$JS.ACK.hengli_to_wsw_test.>","_INBOX.>"]
        }
      }}
    ]
  }
}
EOF

cat <<- EOF > a_2.conf
cluster: {
  name: hengli
  port: 15334
  routes: [nats-route://127.0.0.1:15333,nats-route://127.0.0.1:15334,nats-route://127.0.0.1:15335]
}
http_port: 18223
jetstream: {
  domain: hengli
  store_dir: "./jetstream/hengli_2"
}
leafnodes: {
  port: 14334
  remotes: [{
    account: hengli
    urls: [nats-leaf://_hengliConnUser_:_hengliConnUser_@127.0.0.1:24333,nats-leaf://_hengliConnUser_:_hengliConnUser_@127.0.0.1:24334,nats-leaf://_hengliConnUser_:_hengliConnUser_@127.0.0.1:24335]
  }]
}
port: 14223
server_name: hengli_2

accounts: {
  hengli: {
    jetstream: enabled
    users:[
      {user: _hengliConnUser_, password: _hengliConnUser_},
      {user: hengli, password: hengli, permissions: {
        publish: {
          allow:["hengli.to.wsw.test","\$JS.API.>"]
        },
        subscribe: {
          allow:["\$JS.ACK.hengli_to_wsw_test.>","_INBOX.>"]
        }
      }}
    ]
  }
}
EOF

# Define the server configs for cluster.
cat <<- EOF > "a_3.conf"

cluster: {
  name: hengli
  port: 15335
  routes: [nats-route://127.0.0.1:15333,nats-route://127.0.0.1:15334,nats-route://127.0.0.1:15335]
}
http_port: 18224
jetstream: {
  domain: hengli
  store_dir: "./jetstream/hengli_3"
}
leafnodes: {
  port: 14335
  remotes: [{
    account: hengli
    urls: [nats-leaf://_hengliConnUser_:_hengliConnUser_@127.0.0.1:24333,nats-leaf://_hengliConnUser_:_hengliConnUser_@127.0.0.1:24334,nats-leaf://_hengliConnUser_:_hengliConnUser_@127.0.0.1:24335]
  }]
}
port: 14224
server_name: hengli_3

accounts: {
  hengli: {
    jetstream: enabled
    users:[
      {user: _hengliConnUser_, password: _hengliConnUser_},
      {user: hengli, password: hengli, permissions: {
        publish: {
          allow:["hengli.to.wsw.test","\$JS.API.>"]
        },
        subscribe: {
          allow:["\$JS.ACK.hengli_to_wsw_test.>","_INBOX.>"]
        }
      }}
    ]
  }
}
EOF

# Setup the hub cluster.
cat <<- EOF > "b_1.conf"
http_port: 28222
cluster: {
  name: wsw
  port: 25333
  routes: [nats-route://127.0.0.1:25333,nats-route://127.0.0.1:25334,nats-route://127.0.0.1:25335]
}
jetstream: {
  domain: wsw
  store_dir: "./jetstream/wsw_1"
}
leafnodes: {
  port: 24333
}
port: 24222
server_name: wsw_1

accounts: {
  hengli: {
    jetstream: enabled
    users:[
      {user: _hengliConnUser_, password: _hengliConnUser_}
    ]
    exports:[
      {service: "\$JS.hengli.API.>", response: stream},
      {service: "\$JS.FC.>"},
      {stream: "deliver.hengli.wsw.>", accounts: ["wsw"]}
    ]
  },
  wsw: {
    jetstream: enabled
    users:[
      {user: _wswConnUser_, password: _wswConnUser_},
      {user: wsw, password: wsw, permissions: {
        publish: {
          allow:["\$JS.ACK.hengli_to_wsw_test.>"]
        },
        subscribe: {
          allow:["_recv_wsw.hengli.to.wsw.test"]
        }
      }}
    ]
    imports:[
      {service: {account:"hengli", subject: "\$JS.hengli.API.>"}, to: "\$JS.hengli.wsw.API.>"},
      {service: {account: "hengli", subject: "\$JS.FC.>"}},
      {stream: {account:"hengli", subject:"deliver.hengli.wsw.>"}}
    ]
  }
}
EOF

cat <<- EOF > "b_2.conf"

cluster: {
  name: wsw
  port: 25334
  routes: [nats-route://127.0.0.1:25333,nats-route://127.0.0.1:25334,nats-route://127.0.0.1:25335]
}
jetstream: {
  domain: wsw
  store_dir: "./jetstream/wsw_2"
}
leafnodes: {
  port: 24334
}
port: 24223
server_name: wsw_2

accounts: {
  hengli: {
    jetstream: enabled
    users:[
      {user: _hengliConnUser_, password: _hengliConnUser_}
    ]
    exports:[
      {service: "\$JS.hengli.API.>", response: stream},
      {service: "\$JS.FC.>"},
      {stream: "deliver.hengli.wsw.>", accounts: ["wsw"]}
    ]
  },
  wsw: {
    jetstream: enabled
    users:[
      {user: _wswConnUser_, password: _wswConnUser_},
      {user: wsw, password: wsw, permissions: {
        publish: {
          allow:["\$JS.ACK.hengli_to_wsw_test.>"]
        },
        subscribe: {
          allow:["_recv_wsw.hengli.to.wsw.test"]
        }
      }}
    ]
    imports:[
      {service: {account:"hengli", subject: "\$JS.hengli.API.>"}, to: "\$JS.hengli.wsw.API.>"},
      {service: {account: "hengli", subject: "\$JS.FC.>"}},
      {stream: {account:"hengli", subject:"deliver.hengli.wsw.>"}}
    ]
  }
}
EOF

cat <<- EOF > "b_3.conf"
cluster: {
  name: wsw
  port: 25335
  routes: [nats-route://127.0.0.1:25333,nats-route://127.0.0.1:25334,nats-route://127.0.0.1:25335]
}
jetstream: {
  domain: wsw
  store_dir: "./jetstream/wsw_3"
}
leafnodes: {
  port: 24335
}
port: 24224
server_name: wsw_3

accounts: {
  hengli: {
    jetstream: enabled
    users:[
      {user: _hengliConnUser_, password: _hengliConnUser_}
    ]
    exports:[
      {service: "\$JS.hengli.API.>", response: stream},
      {service: "\$JS.FC.>"},
      {stream: "deliver.hengli.wsw.>", accounts: ["wsw"]}
    ]
  },
  wsw: {
    jetstream: enabled
    users:[
      {user: _wswConnUser_, password: _wswConnUser_},
      {user: wsw, password: wsw, permissions: {
        publish: {
          allow:["\$JS.ACK.hengli_to_wsw_test.>"]
        },
        subscribe: {
          allow:["_recv_wsw.hengli.to.wsw.test"]
        }
      }}
    ]
    imports:[
      {service: {account:"hengli", subject: "\$JS.hengli.API.>"}, to: "\$JS.hengli.wsw.API.>"},
      {service: {account: "hengli", subject: "\$JS.FC.>"}},
      {stream: {account:"hengli", subject:"deliver.hengli.wsw.>"}}
    ]
  }
}
EOF

# ### Bring up the cluster

# Start a server for each configuration and sleep a second in
# between so the seeds can startup and get healthy.
echo "Starting b_1..."
nats-server -c b_1.conf -P b_1.pid > /dev/null 2>&1 &
sleep 1

echo "Starting b_2..."
nats-server -c b_2.conf -P b_2.pid > /dev/null 2>&1 &
sleep 1

echo "Starting b_3..."
nats-server -c b_3.conf -P b_3.pid > /dev/null 2>&1 &
sleep 1

# Wait until the servers up and healthy.
echo 'Hub cluster healthy?'
curl --fail --silent \
  --retry 5 \
  --retry-delay 2 \
  http://localhost:28222/healthz; echo

echo "Starting a_1..."
nats-server -c a_1.conf -P a_1.pid > /dev/null 2>&1 &
sleep 1

echo "Starting a_2..."
nats-server -c a_2.conf -P a_2.pid > /dev/null 2>&1 &
sleep 1

echo "Starting a_3..."
nats-server -c a_3.conf -P a_3.pid > /dev/null 2>&1 &
sleep 1


# Wait until the servers up and healthy.
echo 'Leaf cluster healthy?'
curl --fail --silent \
  --retry 5 \
  --retry-delay 2 \
  http://localhost:18222/healthz; echo

cat <<- EOF > origin-stream.json
{
  "name": "hengli_to_wsw_test",
  "subjects": [
    "hengli.to.wsw.test"
  ],
  "retention": "limits",
  "max_consumers": -1,
  "max_msgs": -1,
  "max_bytes": -1,
  "max_age": 0,
  "max_msgs_per_subject": -1,
  "max_msg_size": -1,
  "discard": "old",
  "storage": "file",
  "num_replicas": 2,
  "duplicate_window": 120000000000,
  "allow_direct": false,
  "mirror_direct": false,
  "sealed": false,
  "deny_delete": false,
  "deny_purge": false,
  "allow_rollup_hdrs": false
}
EOF

cat <<- EOF > mirror-stream.json
{
  "name": "_mirror_hengli_to_wsw_test_hengli",
  "retention": "limits",
  "max_consumers": -1,
  "max_msgs": -1,
  "max_bytes": -1,
  "max_age": 3600000000000,
  "max_msgs_per_subject": -1,
  "max_msg_size": -1,
  "discard": "old",
  "storage": "file",
  "num_replicas": 2,
  "mirror": {
    "name": "hengli_to_wsw_test",
    "external": {
      "api": "\$JS.hengli.wsw.API",
      "deliver": "deliver.hengli.wsw"
    }
  },
  "allow_direct": false,
  "mirror_direct": true,
  "sealed": false,
  "deny_delete": false,
  "deny_purge": false,
  "allow_rollup_hdrs": false
}
EOF

nats -s "nats://_hengliConnUser_:_hengliConnUser_@127.0.0.1:14222" \
  stream add --config origin-stream.json

nats -s "nats://_hengliConnUser_:_hengliConnUser_@127.0.0.1:14222" \
  stream report

nats -s "nats://_wswConnUser_:_wswConnUser_@127.0.0.1:24222" \
  stream add --config mirror-stream.json

nats -s "nats://_wswConnUser_:_wswConnUser_@127.0.0.1:24222" \
  stream report

echo 'Starting bench...'
nats bench \
  -s "nats://hengli:hengli@127.0.0.1:14222" \
  --pub 1 \
  --js \
  --stream hengli_to_wsw_test \
  --msgs 60000 \
  --no-progress \
  hengli.to.wsw.test

nats bench \
  -s "nats://hengli:hengli@127.0.0.1:14222" \
  --pub 1 \
  --js \
  --stream hengli_to_wsw_test \
  --msgs 60000 \
  --no-progress \
  hengli.to.wsw.test

# Report the streams
nats -s "nats://_hengliConnUser_:_hengliConnUser_@127.0.0.1:14222" \
  stream report

nats -s "nats://_wswConnUser_:_wswConnUser_@127.0.0.1:24222" \
  stream report
