require "nats"

# Get the passed NATS_URL or fallback to the default. This can be
# a comma-separated string. We convert it to an `Array(URI)` to pass
# to the NATS client.
servers = ENV.fetch("NATS_URL", "nats://localhost:4222")
  .split(',')
  .map { |url| URI.parse(url) }

# Create a client connection to an available NATS server.
nats = NATS::Client.new(servers)

# When the program exits, we close the NATS client which waits for any pending
# messages (published or in a subscription) to be flushed.
at_exit { nats.close }

# To publish a message, simply provide the _subject_ of the message
# and encode the message payload. NATS subjects are hierarchical using
# periods as token delimiters. `greet` and `joe` are two distinct tokens.
nats.publish "greet.bob", "hello"

# Now we are going to create a subscription and utilize a wildcard on
# the second token. The effect is that this subscription shows _interest_
# in all messages published to a subject with two tokens where the first
# is `greet`.
nats.subscribe "greet.*" do |msg|
  puts "#{String.new(msg.body)} on subject #{msg.subject}"
end

# Let's publish three more messages which will result in the messages
# being forwarded to the local subscription we have.
nats.publish "greet.joe", "hello"
nats.publish "greet.pam", "hello"
nats.publish "greet.sue", "hello"
