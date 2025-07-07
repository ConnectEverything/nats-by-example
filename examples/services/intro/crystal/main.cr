require "nats"
require "nats/services"

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

# ### Defining a service
#
# This will create a service definition. Service definitions are made up of
# the service name (which can't have things like whitespace in it), a version,
# and a description. Even with no running endpoints, this service is discoverable
# via the service protocol and by service discovery tools like `nats service`.
# All of the default background handlers for discovery, PING, and stats are
# started at this point.
service = nats.services.add "minmax",
  version: "0.0.1",
  description: "Returns the min/max number in a request"

puts "Created service: #{service.name} (#{service.id})"

# ### Adding endpoints
#
# Groups serve as namespaces and are used as a subject prefix when endpoints
# don't supply fixed subjects. In this case, all endpoints will be listening
# on a subject that starts with `minmax.`
service.add_group "minmax" do |root|
  # Each endpoint represents a subscription. The supplied handlers will respond
  # to `minmax.min` and `minmax.max`, respectively.

  # Add a `min` endpoint to the service, which returns the minimum
  # value from a JSON-serialized list of integer values.
  root.add_endpoint "min" do |request|
    min = Array(Int64).from_json(request.data_string).min

    nats.reply request, min.to_json
  end

  # Add a `max` endpoint to the service, which returns the maximum
  # value from a JSON-serialized list of integer values.
  root.add_endpoint "max" do |request|
    max = Array(Int64).from_json(request.data_string).max

    nats.reply request, max.to_json
  end
end

# ### Sending a request
#
# Now we create a list of integer values to send to our `minmax.min` and
# `minmax.max` endpoints.
request_data = [-1, 2, 100, -2000]

# Send a request to the `minmax.min` endpoint containing the array above. Note
# that this is just a regular NATS request.
if response = nats.request("minmax.min", request_data.to_json, timeout: 2.seconds)
  puts "Requested min value, got #{response.data_string.to_i}"
end

# Now we do the same thing with our `minmax.max` endpoint.
if response = nats.request("minmax.max", request_data.to_json, timeout: 2.seconds)
  puts "Requested max value, got #{response.data_string.to_i}"
end
