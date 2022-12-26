require "nats"
require "nats/kv"

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

# We create the KV bucket with a history of 10 entries per key. The default
# history is 1, which only stores the current value of each key.
kv = nats.kv.create_bucket("profiles", history: 10)

# We can use the `put` method to set the value of a key and get it back out with
# the `get` method. If a key does not exist, `get` returns `nil`.
kv.put "sue.color", "blue"
if entry = kv.get("sue.color")
  pp key: entry.key, revision: entry.revision, value: String.new(entry.value)
end

# We can change the value by calling `put` again.
kv.put "sue.color", "green"
if entry = kv.get("sue.color")
  pp key: entry.key, revision: entry.revision, value: String.new(entry.value)
end

# The `update` method takes the key, value, and the revision to update _from_.
# If the specified revision is not the current one, the update will fail and
# the `update` method returns `nil`, giving you a mechanism to detect conflicts
# when modifying keys.
puts "Updating sue.color: 'red' from revision 1"
unless kv.update("sue.color", "red", 1)
  puts "Could not update, invalid revision"
end

# When we call `update` with the current revision, it succeeds and returns the
# current revision. We then refetch the current value to check ourselves.
if kv.update("sue.color", "red", 2) && (entry = kv.get("sue.color"))
  pp key: entry.key, revision: entry.revision, value: String.new(entry.value)
end

# `NATS::KV::Bucket` also provides `[]=` and `[]?` convenience methods so you
# can get and set values as if they were a Crystal `Hash`. The assertive `[]`
# method is not provided, however.
kv["sue.color"] = "pink"
if value = kv["sue.color"]?
  pp String.new(value)
end

# You can also watch for changes to a key or key pattern. Here, we watch for
# changes to `"sue.*"` and then grab an `Iterator`.
watch = kv.watch("sue.*")
iterator = watch.each

# Here we set some properties and then call `Iterator#next` to get the entry
# for those keys.
kv["sue.color"] = "purple"
entry = iterator.next.as(NATS::KV::Entry)
pp key: entry.key, revision: entry.revision, value: String.new(entry.value)

kv["sue.food"] = "pizza"
entry = iterator.next.as(NATS::KV::Entry)
pp key: entry.key, revision: entry.revision, value: String.new(entry.value)

# Once we're done watching a key, we call `watch.stop`.
watch.stop

# We can look at all of the keys in a KV store with the `keys` method.
pp keys: kv.keys

# We can also filter the keys with a pattern as we would normal NATS subjects.
kv["nats.kv"] = "pretty great"
pp filtered_keys: kv.keys("nats.*")

# We can also get the history of a key or pattern of keys
puts
puts "History:"
kv.history("sue.*").each do |entry|
  pp key: entry.key, revision: entry.revision, value: String.new(entry.value)
end

# You can replicate the `history` method by passing `include_history: true` to
# `watch` and calling `each` with a block. `NATS::KV::Entry#delta` is how close
# to the end of the key's history you are.
watch = kv.watch("sue.*", include_history: true)
watch.each do |entry|
  pp key: entry.key, revision: entry.revision, value: String.new(entry.value)
  watch.stop if entry.delta == 0
end
