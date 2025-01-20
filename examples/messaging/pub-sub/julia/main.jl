# Import NATS.jl library.
using NATS

# Get the passed NATS_URL or fallback to the default. This can be
# a comma-separated string.
url = get(ENV, "NATS_URL", NATS.DEFAULT_CONNECT_URL)
@info "NATS server url is $url"

# Create a client connection to an available NATS server.
nc = NATS.connect(url)

# To publish a message, simply provide the _subject_ of the message
# and encode the message payload. NATS subjects are hierarchical using
# periods as token delimiters. `greet` and `joe` are two distinct tokens.
publish(nc, "greet.bob", "hello")

# Now we are going to create a subscription and utilize a wildcard on
# the second token. The effect is that this subscription shows _interest_
# in all messages published to a subject with two tokens where the first
# is `greet`.
sub = subscribe(nc, "greet.*") do msg
    @info "$(payload(msg)) on subject $(msg.subject)"
end

# Let's publish three more messages which will result in the messages
# being forwarded to the local subscription we have.
for name in ("joe", "pam", "sue")
    publish(nc, "greet.$name", "hello")
end

# Finally we drain the connection which waits for any pending
# messages (published or in a subscription) to be flushed.
drain(nc)