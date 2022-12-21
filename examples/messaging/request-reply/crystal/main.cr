require "nats"

# Get the passed NATS_URL or fallback to the default. This can be
# a comma-separated string. We convert it to an `Array(URI)` to pass
# to the NATS client.
servers = ENV.fetch("NATS_URL", "nats://localhost:4222")
  .split(',')
  .map { |url| URI.parse(url) }

# Create a client connection to an available NATS server.
nats = NATS::Client.new(servers)
nats.on_error do |ex|
    Log.error { ex }
end

# When the program exits, we close the NATS client which waits for any pending
# messages (published or in a subscription) to be flushed.
at_exit { nats.close }

subscription = nats.subscribe "greet.*" do |msg|
  name = msg.subject[6..]
  nats.reply msg, "hello, #{name}"
end

if response = nats.request("greet.joe", "", timeout: 500.milliseconds)
  puts String.new(response.body)
end

if response = nats.request("greet.sue", "", timeout: 500.milliseconds)
  puts String.new(response.body)
end

if response = nats.request("greet.bob", "", timeout: 500.milliseconds)
  puts String.new(response.body)
end

subscription.close

unless response = nats.request("greet.joe", "", timeout: 1.second)
  puts "No response"
end
