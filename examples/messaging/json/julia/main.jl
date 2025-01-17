# Import NATS.jl package.
using NATS
using JSON3

# Get the passed NATS_URL or fallback to the default. This can be
# a comma-separated string.
url = get(ENV, "NATS_URL", NATS.DEFAULT_CONNECT_URL)
@info "NATS server url is $url"

struct Payload
    foo::String
    bar::Int64
end

# Create a client connection to an available NATS server.
nc = NATS.connect(url)

# Construct a Payload value and serialize it.
data_object = Payload("bar", 27)
data_json = JSON3.write(data_object)

# Create a subscription that receives typed argument. For message
# that contain invalid payload error will be reported in a separate
# monitoring task. Alternatively untyped parameter can be used with
# user provided deserialization to allow handle invalid data in a
# prefered way.
sub = subscribe(nc, "foo") do msg::JSON3.Object
    @info "Received json from typed subscription: $msg"
end

# Publish the serialized payload.
publish(nc, "foo", data_json)
publish(nc, "foo", "not a json")
publish(nc, "foo", "also not a json")

# Wait for error to be reported from handlers.
# To avoid excessive console output errors from subscrption handlers
# are batched and reported every few seconds.
sleep(10)

# Close connection.
drain(nc)
