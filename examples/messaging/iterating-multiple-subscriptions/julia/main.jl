# Import NATS.jl package.
using NATS;

# Get the passed NATS_URL or fallback to the default. This can be
# a comma-separated string.
url = get(ENV, "NATS_URL", NATS.DEFAULT_CONNECT_URL)
@info "NATS server url is $url"

# Create a client connection to an available NATS server.
nc = NATS.connect(url)

# Specify list of subjects to listen on.
subjects = ["s1", "s2", "s3", "s4"]

# In Julia Channel is a way to establish communication between tasks.
# See [Channel documentation](https://docs.julialang.org/en/v1/manual/asynchronous-programming/#Communicating-with-Channels)
# for more details.
ch = Channel()

# Create subscriptions for all subjects, subscription handler
# puts received message into channel. 
subs = map(subjects) do subject
    subscribe(nc, subject) do msg
        put!(ch, msg)
    end
end

# Create consumer task consuming messages from the channel.
subscribe_task = Threads.@spawn while true
    msg = take!(ch)
    @info "Received $(payload(msg)) from subject $(msg.subject)"
end

# Create consumer task publishing messages.
# It sends numbers from 1 to 5 to every subject.
publish_task = Threads.@spawn for i in 1:5
    for subject in subjects
        publish(nc, subject, string(i))
    end
    sleep(0.5)
end

# Wait for all publications are done.
wait(publish_task)

# Close all subscriptions, `drain` will wait until all messages
# in buffers are processed.
for sub in subs
    drain(nc, sub)
end

# Finally close the channel to force consumer task to complete.
close(ch)

# At this moment both spawned tasks should be completed.
# Let's make sure it is true, dangling tasks might cause resource leak.
@show istaskdone(publish_task)
@show istaskdone(subscribe_task)
