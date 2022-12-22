require "nats"
require "json"

# Use the `JSON::Serializable` mixin to be able to serialize and deserialize a
# struct we define.
struct Payload
  include JSON::Serializable

  getter foo : String
  getter bar : UInt8

  def initialize(@foo, @bar)
  end
end

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

# Create a subscription. We will receive both valid and invalid JSON here, so we
# rescue the `JSON::ParseException`.
nats.subscribe "foo" do |msg|
  json = String.new(msg.body)
  payload = Payload.from_json(json)

  puts "received valid JSON: #{payload}"
rescue JSON::ParseException
  puts "received invalid JSON: #{json}"
end

# Construct and serialize a `Payload` struct.
payload = Payload.new(foo: "bar", bar: 27).to_json

# Publish the serialized payload
nats.publish "foo", payload
nats.publish "foo", "not json"
