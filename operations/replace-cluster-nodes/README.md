# Replace cluster nodes

This example shows an operational process of rotating nodes in and out. The original use case was to replace the underlying hardware of existing nodes in a cluster. The goal is to minimize or remove the need for downtime.

[Watch a demo](https://youtu.be/iFmJ0m1wjY8) of this migration.

The general algorithm works as follows:

- Given nodes `n0`, `n1`, and `n2`
- Add node `n3`
  - `scripts/add-node.sh n3`
- Put `n0` in lame duck mode
  - `scripts/signal-lameduck.sh n0`
- Remove `n0` which forces replicas to be scheduled on `n3`
  - `scripts/remove-peer.sh n0`
- Repeat for the following pairs
  - `n4` replaces `n1`
  - `n0` (new hardware/configuration) replaces `n2`
  - `n1` (new hardware/configuration) replaces `n3`
  - `n2` (new hardware/configuration) replaces `n4`

This cycle is required so that the original connection URLs used by the clients remain addressable for if/when they need to reconnect. Otherwise if they were all new hostnames or ports, clients would need to update their local configuration.

## Try locally

*Ensure you are in this directory when you run the scripts.*

### Dependencies

- The [`nats-sever`](https://github.com/nats-io/nats-server) binary
- The [`nats`](https://github.com/nats-io/natscli) CLI
- The `watch` command
  - Native to Linux, use `brew install watch` on macOS

### Steps

- Ensure your `nats context` is set to localhost without any creds
  - If you never used `nats context`, then you should be good
- Start the cluster
  - `./scripts/start-cluster.sh`
  - Servers are run in the background
  - PIDs are in `pids/`
  - Logs are written to `logs/`
  - Data written to `data/`
  - Watch the servers using `./scripts/watch-servers.sh` in a separate shell
- Create the streams
  - `./scripts/create-streams.sh`
  - Creates two streams `foo` and `bar` matching all subjects under their names, e.g. `foo.>`
  - One with file-based storage, unlimited everything, 3 replicas
  - One with memory-based storage, max of 100k messages, 3 replicas
  - Watch the streams using `./scripts/watch-streams.sh`
- Create the consumers
  - `./scripts/create-consumers.sh`
  - Creates to push consumesr `foo-c` and `bar-c` to subjects `foo-c` and `bar-c`
  - Watch each consumer using `./scripts/watch-foo-consumer.sh` and `./scripts/watch-bar-consumer.sh`
- Start publishers for each stream
  - In separate shells, `./scripts/start-foo-publisher.sh` and `./scripts/start-bar-publisher.sh`
- Start subscribers for each consumers
  - In separate shells, `./scripts/start-foo-subscriber.sh` `./scripts/start-bar-subscriber.sh`
- Run the migration!
  - `./scripts/migrate-cluster.sh`
  - This script choose a fairly aggressive wait time (for demo purpsoes) of only 60 seconds for lame-ducking a node and then another 60 seconds for removing a node before the next node is migrated. Feel free to edit the script to test other timings. In practice, the wait time would be dependent on how much data needs to be migrated.
  - Observe the behavior of the various clients
  - Some will disconnect and reconnect automatically
  - There may be some slow down on consumption
- Stop the cluster
  - `./scripts/stop-cluster.sh`
