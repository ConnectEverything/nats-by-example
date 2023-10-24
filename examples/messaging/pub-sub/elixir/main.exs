# Set up the dependencies for this script. Ordinarily you would have this set of dependencies
# declared in your `mix.exs` file.
Mix.install([
  # For documentation on the Gnat library, see https://hexdocs.pm/gnat/readme.html
  {:gnat, "~> 1.6"},
  {:jason, "~> 1.0"}
])

url = System.get_env("NATS_URL", "nats://127.0.0.1:4222")
uri = URI.parse(url)

# Call `start_link` on `Gnat` to start the Gnat application supervisor
{:ok, gnat} = Gnat.start_link(%{host: uri.host, port: uri.port})

# Manual subscriptions are easy and straightforward, just supply a topic and the target
# pid to receive the `{:msg, m}` messages from the subscription.
{:ok, subscription} = Gnat.sub(gnat, self(), "nbe.*")
# Here we send a message to a subject that has a subscriber
:ok = Gnat.pub(gnat, "nbe.news", "NATS by example is a great learning resource!")

# In elixir, an explicit `receive` call blocks the current process until a message
# arrives in its inbox. In this case, we're waiting for the `{:msg, m}` tuple from
# the NATS client.
receive do
  {:msg, %{body: body, topic: "nbe.news", reply_to: nil}} ->
    IO.puts("Manual subscription received: '#{body}'")
end

# Now let's move on to more resilient and production-grade ways of subscribing

# In addition to one-off subscriptions, you can create a resilient consumer
# supervisor that you intend to keep running for a long period of time that
# will survive network partition events. This consumer supervisor can invoke
# callbacks in a module that conforms to the `Gnat.Server` behavior, like this
# `DemoServer` module.
defmodule DemoServer do
  use Gnat.Server

  def request(%{body: body, topic: topic}) do
    IO.puts("Received message on '#{topic}': '#{body}'")
    :ok
  end

  # The error handler is an optional callback. `Gnat.Server` has a default one that you
  # can use.
  def error(%{gnat: gnat, reply_to: reply_to}, _error) do
    Gnat.pub(gnat, reply_to, "Something went wrong and I can't handle your message")
  end
end

# The `Gnat.ConnectionSupervisor` is a process that monitors your NATS connection. If connection
# is lost, this process will retry according to its backoff settings to re-establish a connection.
gnat_supervisor_settings = %{
  name: :gnat,
  backoff_period: 4_000,
  connection_settings: [
    %{host: uri.host, port: uri.port}
  ]
}
{:ok, _conn} = Gnat.ConnectionSupervisor.start_link(gnat_supervisor_settings)

# The connection supervisor's `start_link` establishes a connection asynchronously, so we need to
# delay here until the connection is running. This isn't normally a problem when putting connection
# supervisors into a supervision tree at startup
if Process.whereis(:gnat) == nil do
  Process.sleep(300)
end

# Consumer supervisors work in tandem with connection supervisors. The `connection_name` setting
# refers to the name of a supervised connection, and not the `Gnat` application.
consumer_supervisor_settings = %{
  connection_name: :gnat,
  # This is the module name of a module that exhibits the `Gnat.Server` behavior
  module: DemoServer,
  # We can subscribe on multiple topics, each of which can have wildcards
  subscription_topics: [
    %{topic: "rpc.demo", queue_group: "demo"},
  ],
}

# In most applications the connection and consumer supervisors are started as part of the
# supervision tree, but for this sample we just create it manually via `start_link`.
{:ok, _sup} = Gnat.ConsumerSupervisor.start_link(consumer_supervisor_settings)
IO.puts("Started consumer supervisor")

# This publishes on the topic on which our consumer supervisor is listening.
Gnat.pub(:gnat, "rpc.demo", "hello")
