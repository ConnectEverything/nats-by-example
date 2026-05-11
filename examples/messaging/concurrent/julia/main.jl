# Import NATS.jl package.
using NATS;

# Get the passed NATS_URL or fallback to the default. This can be
# a comma-separated string.
url = get(ENV, "NATS_URL", NATS.DEFAULT_CONNECT_URL)
@info "NATS server url is $url"

# Create a client connection to an available NATS server.
nc = NATS.connect(url)

# Subscribe to a subject and start waiting for messages in the background and
# start processing each message in a separate task. Handler sleeps for some random
# time up to 1 second to simulate doing some work.
subscribe(nc, "numbers"; spawn = true) do msg
    sleep(rand())
    @info "Received $(payload(msg))"
end

# Publish 50 messages at the same time. Despite expected total processing time is 25
# seconds due to parallel setup of subscription everything completes in less than second.
for i in 1:50
    publish(nc, "numbers", string(i))
end

# Close connection.
drain(nc)