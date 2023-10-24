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


# Now let's set up a consumer supervisor. Instead of manually specifying subjects
# on which to subscribe, we'll supply the metadata required to expose
# the service to consumers.
consumer_supervisor_settings = %{
  connection_name: :gnat,
  # This is the name of a module defined below
  module: ExampleService,
  service_definition: %{
    name: "exampleservice",
    description: "This is an example service",
    # This service version needs to conform to the semver specification
    version: "0.1.0",
    endpoints: [
      # Each endpoint has a mandatory name, an optional group, and optional metadata
      %{
        name: "add",
        group_name: "calc",
      },
      %{
        name: "sub",
        group_name: "calc"
      }
    ]
  }
}

# In this service definition, we have a single service, `exampleservice`. It has two endpoints, `add` and `sub`, each of which
# belong to the `calc` group. This means that the service will default to responding on `calc.add` and `calc.sub`. Let's create the
# consumer supervisor for this
{:ok , _pid} = Gnat.ConsumerSupervisor.start_link(consumer_supervisor_settings)


# This is a module that conforms to the `Gnat.Services.Server` behavior. The name of this module
# matches the `module` field in the consumer supervisor settings.
defmodule ExampleService do
  use Gnat.Services.Server

  # This handler is matching just on the subject
  def request(%{topic: "calc.add", body: body}, _endpoint, _group) do
    IO.puts "Calculator adding...#{inspect(body)}"
    {:reply, "42"}
  end

  # Simulate an error occurring in a handler
  def request(%{body: body}, _, _) when body == "failthis" do
    {:error, "woopsy"}
  end

  # This handler matches on endpoint and group respectively
  def request(%{body: body}, "sub", "calc") do
    IO.puts "Calculator subtracting...#{inspect(body)}"

    {:reply, "24"}
  end

  # In case of an error, we can manually craft a response (remember this is a NATS reply)
  def error(_msg, e) do
    {:reply, "service error: #{inspect(e)}"}
  end
end


# Let's invoke the service a few times to generate some statistics.
{:ok, %{body: res}} = Gnat.request(:gnat, "calc.add", "add this!")
IO.puts("Add result: #{res}")
{:ok, %{body: res2}} = Gnat.request(:gnat, "calc.sub", "subtract this!")
IO.puts("Subtract result: #{res2}")

# This simulates invoking an endpoint that returned an error
res3 = Gnat.request(:gnat, "calc.sub", "failthis")
IO.puts("Fail result: #{inspect(res3)}")

# Get service stats. When you scroll down to the output of this demo, you'll see that each
# of the endpoints has a request count of 1, and the sub endpoint has an error count of 1,
# and all of the endpoints have been keeping track of execution time
{:ok, %{body: stats}} = Gnat.request(:gnat, "$SRV.STATS.exampleservice", "")
jstats = Jason.decode!(stats)

IO.puts("Service Stats:")
IO.inspect(jstats)
:timer.sleep(50)
