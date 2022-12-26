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
  max_msgs: 10_i64,
)

# We publish 11 messages to our 10-message stream, so we will exceed our limit.
# As mentioned above, this will discard older messages when new ones come in.
event_types = %w[
  page_loaded
  mouse_clicked
  input_focused
]
11.times do
  type = event_types.sample
  subject = "events.#{type}"
  puts "Publishing #{subject}..."
  js.publish(subject, "")
end

# When we fetch the current stream state from the server, we see that there are
# indeed only 10 messages in the stream, with sequence numbers 2..11. This
# indicates that the first message in our stream with a capacity of 10 was
# dropped to make room for the 11th message.
name = stream.config.name
if stream = js.stream.info(name)
  pp stream.state
else
  raise "Could not find stream: #{name}"
end
