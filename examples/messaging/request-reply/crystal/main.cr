require "nats"

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

# In addition to vanilla publish-subscribe, NATS supports request-reply
# interactions as well. Under the covers, this is just an optimized
# pair of publish-subscribe operations.
#
# The _request handler_ is just a subscription that _replies_ to a message
# sent to it. This kind of subscription is called a _service_.
#
# For this example, we use the built-in asynchronous subscription in the Crystal
# client. When the NATS server receives a message that matches the pattern
# `greet.*`, a copy of it will be yielded to this block.
#
# We are also storing the `NATS::Subscription` in the `subscription` local
# variable, which we can use to unsubscribe later.
subscription = nats.subscribe "greet.*" do |msg|
  _, name = msg.subject.split('.')
  nats.reply msg, "hello, #{name}"
end

# Now we can use the built-in `NATS::Client#request` method to send requests.
# We simply pass an empty body since that is not being used right now. We can
# also specify a timeout with a request. If we don't specify it, the default
# is 2 seconds.
%w[joe sue bob].each do |name|
  if response = nats.request("greet.#{name}", "", timeout: 500.milliseconds)
    puts String.new(response.body)
  end
end

# What happens if the service is _unavailable_? We can simulate this by
# unsubscribing our handler from above. Now if we make a request, we won't
# get a response.
subscription.close

unless response = nats.request("greet.joe", "", timeout: 1.second)
  puts "No response"
end
