# NATS Zero Down

Exploration of strategies for migrating a JetStream-enabled NATS cluster or individual streams with zero downtime.

- [Replacing nodes in a cluster](#replacing-nodes-in-a-cluster)
- Replacing a single node
- Moving stream data to a different node

### Replacing nodes in a cluster

This was the driving use case of the repository. I had a cluster deployed on disks that were running out of space and needed to migrate them to new nodes with larger disks. The goal was to do this without interrupting client connections. Reads and writes should still continue.

Check out the [replace-cluster-nodes](./replace-cluster-nodes) directory for the general steps and the set of scripts used to run this example locally.

[Watch a demo](https://youtu.be/iFmJ0m1wjY8) of this migration.
