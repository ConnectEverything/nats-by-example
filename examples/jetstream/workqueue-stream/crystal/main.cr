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

# ### Create the stream
#
# Define the stream configuration, specifying `:workqueue` for the retention
# policy.
stream = js.stream.create(
  name: "EVENTS",
  retention: :workqueue,
  subjects: %w[events.>],
  storage: :file,
)
puts "Created the stream"

# ### Queue messages
#
# Publish a few messages.
js.publish "events.us.page_loaded", ""
js.publish "events.eu.mouse_clicked", ""
js.publish "events.us.input_focused", ""
puts "Published 3 messages"

# Checking the stream info, we see three messages have been queued.
puts
puts "Stream info without any consumers:"
pp js.stream.info(stream.config.name).try(&.state)

# ### Adding a consumer
#
# Now let's add a consumer and publish a few more messages. It can be
# a [push][push] or [pull][pull] consumer. For this example, we are
# defining a pull consumer.
# [push]: /examples/jetstream/push-consumer/crystal
# [pull]: /examples/jetstream/pull-consumer/crystal
consumer1 = js.consumer.create(
  stream_name: stream.config.name,
  durable_name: "processor-1",
)
sub1 = js.pull_subscribe(consumer1)

# Fetch and ack the queued messages
msgs = sub1.fetch(3)
msgs.each { |msg| js.ack_sync msg }

# Checking the stream info again, we will notice no messages are available.
puts "Stream info with 1 consumer:"
pp js.stream.info(stream.config.name).try(&.state)

# ### Exclusive non-filtered consumer
#
# As noted in the description above, work-queue streams can only have at most one consumer with interest on a subject at any given time. Since the pull consumer above is not filtered, if we try to create another one, it will fail.
begin
  puts
  puts "Creating an overlapping consumer"
  consumer2 = js.consumer.create(
    stream_name: stream.config.name,
    durable_name: "processor-2",
  )
rescue ex
  puts "** #{ex}"
end

# However, if we delete the first one, we can add the new one.
puts
puts "Deleting first consumer"
js.consumer.delete consumer1

puts "Creating second consumer:"
pp consumer2 = js.consumer.create(
  stream_name: stream.config.name,
  durable_name: "processor-2",
)
js.consumer.delete consumer2

# ### Multiple filtered consumers
#
# To create multiple consumers, a subject filter needs to be applied. For this
# example, we could scope each consumer to the geo that the event was published
# from, in this case `us` or `eu`.
puts
puts "Creating non-overlapping consumers"
consumer1 = js.consumer.create(
  stream_name: stream.config.name,
  durable_name: "processor-us",
  filter_subject: "events.us.>",
)
consumer2 = js.consumer.create(
  stream_name: stream.config.name,
  durable_name: "processor-eu",
  filter_subject: "events.eu.>",
)
sub1 = js.pull_subscribe(consumer1)
sub2 = js.pull_subscribe(consumer2)

js.publish("events.eu.mouse_clicked", "")
js.publish("events.us.page_loaded", "")
js.publish("events.us.input_focused", "")
js.publish("events.eu.page_loaded", "")
puts "Published 4 messages"

sub1.fetch(2).each do |msg|
  puts "US subscription got: #{msg.subject}"
  js.ack msg
end
sub2.fetch(2).each do |msg|
  puts "EU subscription got: #{msg.subject}"
  js.ack msg
end
