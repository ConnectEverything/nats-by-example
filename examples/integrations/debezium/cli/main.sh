#!/bin/bash

set -euo pipefail

# Ensure all the services are up and running.
until nc -z nats 4222; do sleep 1; done
until nc -z postgres 5432; do sleep 1; done
until nc -z debezium 8080; do sleep 1; done

# Allow Debezium to setup its connection to Postgres.
sleep 1

# Create a table and insert, update, and delete some data.
printf 'Create and populate the database.\n'
psql -h postgres -c "create table profile (id serial primary key, name text, color text);"

psql -h postgres -c "insert into profile (name, color) values ('Joe', 'blue');"
psql -h postgres -c "insert into profile (name, color) values ('Pam', 'green');"

psql -h postgres -c "update profile set color = 'red' where name = 'Joe';"
psql -h postgres -c "update profile set color = 'yellow' where name = 'Pam';"

psql -h postgres -c "delete from profile where name = 'Joe';"

psql -h postgres -c "create table books (id serial primary key, title text, author text);"

psql -h postgres -c "insert into books (title, author) values ('NATS Diaries', 'Pam');"

# Ensure the change events have been published to the stream.
sleep 1

# Print the stream subjects, seeing each schema/table pair as subjects.
printf '\nStream subjects.\n'
nats stream subjects DebeziumStream

# Print the data in the stream, plucking out the before and after values
# for ease of reading.
printf '\nChange events.\n'
nats stream view --raw DebeziumStream 2> /dev/null | jq -r '.payload'
