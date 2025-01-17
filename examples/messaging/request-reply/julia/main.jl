# Import NATS.jl package
using NATS

# Get the passed NATS_URL or fallback to the default. This can be
# a comma-separated string.
url = get(ENV, "NATS_URL", NATS.DEFAULT_CONNECT_URL)
@info "NATS server url is $url"

# Create a client connection to an available NATS server.
nc = NATS.connect(url)

# In addition to vanilla publish-request, NATS supports request-reply
# interactions as well. Under the covers, this is just an optimized
# pair of publish-subscribe operations.
# The _request handler_ is just a subscription that _responds_ to a message
# sent to it. This kind of subscription is called a _service_.
sub = reply(nc, "greet.*") do msg
    name = last(split(msg.subject, "."))
    "hello, $name"
end

# Now we can use the built-in `request` method to do the service request.
# A payload is optional, and we skip setting it right now. In addition,
# you can specify an optional timeout, but we'll use the default for now.
rep = request(nc, "greet.joe");
@info payload(rep)

# here put a payload
rep = request(nc, "greet.sue", "hello!");
@info payload(rep)
# console.log(rep.string());

# and here we set a timeout of 3 seconds
# rep = request(nc, "greet.bob");
# console.log(rep.string());

# What happens if the service is _unavailable_? We can simulate this by
# unsubscribing our handler from above. Now if we make a request, we will
# expect an error.
unsubscribe(nc, sub);

try
    request(nc, "greet.joe")
catch err
    if err isa NATSError && err.code == 503
        @error "No responders are listening"
    else
        rethrow()
    end
end

# Close the connection
drain(nc)
