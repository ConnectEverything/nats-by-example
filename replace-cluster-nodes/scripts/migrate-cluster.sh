#!/bin/bash

set -euo pipefail

echo 'Migrating n0 -> n3'
echo 'Adding node n3'
scripts/add-node.sh n3

echo 'Signaling lame duck on n0'
scripts/signal-lameduck.sh n0
sleep 60

echo 'Removing n0'
scripts/remove-peer.sh n0
sleep 60


echo 'Migrating n1 -> n4'
echo 'Adding node n4'
scripts/add-node.sh n4

echo 'Signaling lame duck on n1'
scripts/signal-lameduck.sh n1
sleep 60

echo 'Removing n1'
scripts/remove-peer.sh n1
sleep 60


echo 'Migrating n2 -> n0'
echo 'Adding node n0'
scripts/add-node.sh n0

echo 'Signaling lame duck on n2'
scripts/signal-lameduck.sh n2
sleep 60

echo 'Removing n2'
scripts/remove-peer.sh n2
sleep 60


echo 'Migrating n3 -> n1'
echo 'Adding node n1'
scripts/add-node.sh n1

echo 'Signaling lame duck on n3'
scripts/signal-lameduck.sh n3
sleep 60

echo 'Removing n3'
scripts/remove-peer.sh n3
sleep 60


echo 'Migrating n4 -> n2'
echo 'Adding node n2'
scripts/add-node.sh n2

echo 'Signaling lame duck on n4'
scripts/signal-lameduck.sh n4
sleep 60

echo 'Removing n4'
scripts/remove-peer.sh n4
