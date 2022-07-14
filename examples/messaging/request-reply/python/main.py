import os
import asyncio

import nats
from nats.errors import TimeoutError, NoRespondersError

# Get the list of servers.
servers = os.environ.get("NATS_URL", "nats://localhost:4222").split(",")

async def main():
    # Create the connection to NATS which takes a list of servers.
    nc = await nats.connect(servers=servers)

    # In addition to vanilla publish-request, NATS supports request-reply
    # interactions as well. Under the covers, this is just an optimized
    # pair of publish-subscribe operations.
    # The _request handler_ is a subscription that _responds_ to a message
    # sent to it. This kind of subscription is called a _service_.
    # We can use the `cb` argument for asynchronous handling.
    async def greet_handler(msg):
        # Parse out the second token in the subject (everything after
        # `greet.`) and use it as part of the response message.
        name = msg.subject[6:]
        reply = f"hello, {name}"
        await msg.respond(reply.encode("utf8"))

    sub = await nc.subscribe("greet.*", cb=greet_handler)

    # Now we can use the built-in `request` method to do the service request.
    # We simply pass a empty body since that is being used right now.
    # In addition, we need to specify a timeout since with a request we
    # are _waiting_ for the reply and we likely don't want to wait forever.
    rep = await nc.request("greet.joe", b'', timeout=0.5)
    print(f"{rep.data}")

    rep = await nc.request("greet.sue", b'', timeout=0.5)
    print(f"{rep.data}")

    rep = await nc.request("greet.bob", b'', timeout=0.5)
    print(f"{rep.data}")

    # What happens if the service is _unavailable_? We can simulate this by
    # unsubscribing our handler from above. Now if we make a request, we will
    # expect an error.
    await sub.drain()

    try:
        await nc.request("greet.joe", b'', timeout=0.5)
    except NoRespondersError:
        print("no responders")

    await nc.drain()


if __name__ == '__main__':
    asyncio.run(main())
