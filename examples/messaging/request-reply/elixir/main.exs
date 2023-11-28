# Set up the dependencies for this script. Ordinarily you would have this set of dependencies
# declared in your `mix.exs` file.
Mix.install([
  # For documentation on the Gnat library, see https://hexdocs.pm/gnat/readme.html
  {:gnat, "~> 1.7.1"},
  {:jason, "~> 1.0"}
])

url = System.get_env("NATS_URL", "nats://0.0.0.0:4222")
uri = URI.parse(url)

# Call `start_link` on `Gnat` to start the Gnat application supervisor
{:ok, gnat} = Gnat.start_link(%{host: uri.host, port: uri.port})

# The `Gnat.ConnectionSupervisor` is a process that monitors your NATS connection. If connection
# is lost, this process will retry according to its backoff settings to re-establish a connection.
# You can communicate with NATS without this, but we recommend using supervised connections
# and consumers
gnat_supervisor_settings = %{
  name: :gnat,
  backoff_period: 4_000,
  connection_settings: [
    %{host: uri.host, port: uri.port}
  ]
}

{:ok, _conn} = Gnat.ConnectionSupervisor.start_link(gnat_supervisor_settings)

# Give the connection time to establish (this is only needed for these examples and not in production)
if Process.whereis(:gnat) == nil do
  Process.sleep(300)
end


# Now let's set up a consumer supervisor. We use the `subscription_topics` field in the configuration
# map to set up a list of subscriptions which can optionally have queue names.
consumer_supervisor_settings = %{
  connection_name: :gnat,
  # This is the name of a module defined below
  module: ExampleService,
  subscription_topics: [
    %{topic: "request.demo"},
  ]

}

# Starting the consumer supervisor will create the subscription and will monitor
# the connection to re-establish subscriptions in case of failure
{:ok , _pid} = Gnat.ConsumerSupervisor.start_link(consumer_supervisor_settings)


# This is a module that conforms to the `Gnat.Server` behavior. The name of this module
# matches the `module` field in the consumer supervisor settings.
defmodule ExampleService do
  use Gnat.Server

  # This handler will simulate an error
  def request(%{topic: "request.demo", body: body}) when body == "failure" do
    {:error, "something went wrong!"}
  end

  # This handler is matching just on the subject
  def request(%{topic: "request.demo", body: body}) do
    IO.puts "Demo request received message: #{inspect(body)}"
    {:reply, "This is a demo"}
  end

   # defining an error handler is optional, the default one will just call Logger.error for you
   def error(%{gnat: gnat, reply_to: reply_to}, error) do
    Gnat.pub(gnat, reply_to, "An error occurred: #{inspect(error)}")
  end
end

# Now that we know there's an active subscription on the `request.demo` subject, we can
# make a request and process the reply using `Gnat.request`
{:ok, %{body: res}} = Gnat.request(:gnat, "request.demo", "input data")
IO.puts("First result: #{res}")

{:ok, %{body: res}} = Gnat.request(:gnat, "request.demo", "more data")
IO.puts("Second result: #{res}")

# Now cause a failure so we can see error responses
{:ok, %{body: res}} = Gnat.request(:gnat, "request.demo", "failure")
IO.puts("Failure result: #{res}")
