require "nats"
require "nats/jetstream"
require "log"

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

stream = js.stream.create(
  name: "EVENTS",
  subjects: %w[events.>],
  storage: :file,
  max_msgs: 10_i64,
)

event_types = %w[
  page_loaded
  mouse_clicked
  input_focused
]
11.times do
  type = event_types.sample
  subject = "events.#{type}"
  Log.info { "Publishing #{subject}..." }
  js.publish(subject, "")
end

name = stream.config.name
if stream = js.stream.info(name)
  Log.info { stream.state }
else
  raise "Could not find stream: #{name}"
end
