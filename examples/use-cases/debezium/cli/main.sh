#!/bin/bash

set -euo pipefail

# Ensure all the services are up and running.
until nc -z nats 4222; do sleep 1; done
until nc -z postgres 5432; do sleep 1; done
until nc -z debezium 8080; do sleep 1; done

# Create a table and insert, update, and delete some data.
psql -h postgres -c "create table test (id serial primary key, name text, color text);"

psql -h postgres -c "insert into test (name, color) values ('joe', 'blue');"
psql -h postgres -c "insert into test (name, color) values ('pam', 'green');"

psql -h postgres -c "update test set color = 'red' where name = 'joe';"
psql -h postgres -c "update test set color = 'yellow' where name = 'pam';"

psql -h postgres -c "delete from test where name = 'joe';"

# Ensure the data is in the stream.
sleep 1

# Print the data in the stream.
nats stream view --raw DebeziumStream | jq '{before: .payload.before, after: .payload.after}'
