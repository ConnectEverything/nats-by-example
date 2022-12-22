require "nats"
require "nats/jetstream"

# Get the `NATS_URL` from the environment or fallback to the default. This can
# be a comma-separated string. We convert it to an `Array(URI)` to pass to the
# NATS client.
servers = ENV.fetch("NATS_URL", "nats://localhost:4222")
  .split(',')
  .map { |url| URI.parse(url) }

# Create a client connection to an available NATS server.
nats = NATS::Client.new(servers)

# When the program exits, we close the NATS client which waits for any pending
# messages (published or in a subscription) to be flushed.
at_exit { nats.close }

js = nats.jetstream

# Here we create the `EVENTS` stream that listens on all subjects matching
# `events.>` (all subjects starting with `events.`), stored on the filesystem
# for durability, and can contain up to 10 messages. The default discard policy
# will discard old messages when we exceed that limit.
stream = js.stream.create(
  name: "EVENTS",
  subjects: %w[events.>],
  storage: :file,
)

# We're going to publish 5 messages whose subjects match the subjects list for
# our stream, so they will be inserted into the stream.
5.times do |i|
  puts "Publishing events.#{i + 1}"
  js.publish "events.#{i + 1}", ""
end

# We can confirm that we inserted 5 events into the stream by fetching the
# latest stream state and inspecting its state.
if stream = js.stream.info(stream.config.name)
  pp stream.state
else
  raise "Could not fetch stream"
end

# Here we create the consumer we're going to use for our pull subscriptions. We
# set the `durable_name` to ensure the consumer will be persisted if the NATS
# server goes offline.
consumer = js.consumer.create(
  stream_name: stream.config.name,
  durable_name: "EVENTS-pull",
)

# Next we create the pull subscription to the consumer.
pull = js.pull_subscribe(consumer)

# We can use the `fetch` method with no arguments to fetch a single method. We
# can supply a `timeout` argument to tell NATS how long we want to wait, and the
# default is 2 seconds. We also make sure to acknowledge the message with the
# `ack` method.
if msg = pull.fetch
  puts "Got a single message"
  pp msg
  js.ack msg
else
  puts "No messages in the queue"
end

# We can also return a batch of messages by passing a batch size to `fetch` to
# fetch up to that many messages. If there are _any_ messages at all available,
# up to that many will be returned immediately. It will not wait for the timeout
# before returning unless there are no messages.
msgs = pull.fetch(3)
puts "got #{msgs.size} messages"
msgs.each do |msg|
  puts msg
  js.ack msg
end

# This example demonstrates fetching more messages than there are available. We
# specify a timeout of 1 second but since there is 1 message remaining, it
# returns that 1 message immediately. We also use `ack_sync` to acknowledge the messages in this batch.
msgs = pull.fetch(100, timeout: 1.second)
puts "got #{msgs.size} messages"
msgs.each do |msg|
  puts msg
  js.ack_sync msg
end

# We can check the consumer's current state with the `nats.jetstream.consumer.info`
# method, passing in the names of the stream and the consumer. In the output, we
# see we have acknowledged all 5 messages we inserted into the stream above.
pp js.consumer.info(consumer.stream_name, consumer.name)
