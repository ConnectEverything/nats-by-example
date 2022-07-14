import os
import asyncio

import nats
from nats.errors import TimeoutError

# Get the list of servers.
servers = os.environ.get("NATS_URL", "nats://localhost:4222").split(",")

async def main():
    # Create the connection to NATS which takes a list of servers.
    nc = await nats.connect(servers=servers)

	# Messages are published to subjects. Although there are no subscribers,
	# this will be published successfully.
    await nc.publish("greet.joe", b"hello")

	# Let's create a subscription on the greet.* wildcard.
    sub = await nc.subscribe("greet.*")

	# For a synchronous subscription, we need to fetch the next message.
	# However.. since the publish occured before the subscription was
	# established, this is going to timeout.
    try:
        msg = await sub.next_msg(timeout=0.1)
    except TimeoutError:
        pass

	# Publish a couple messages.
    await nc.publish("greet.joe", b"hello")
    await nc.publish("greet.pam", b"hello")

	# Since the subscription is established, the published messages will
	# immediately be broadcasted to all subscriptions. They will land in
    # their buffer for subsequent NextMsg calls.
    msg = await sub.next_msg(timeout=0.1)
    print(f"{msg.data} on subject {msg.subject}")

    msg = await sub.next_msg(timeout=0.1)
    print(f"{msg.data} on subject {msg.subject}")

    # One more for good measures..
    await nc.publish("greet.bob", b"hello")

    msg = await sub.next_msg(timeout=0.1)
    print(f"{msg.data} on subject {msg.subject}")

    # Drain the subscription and connection. In contrast to `unsubscribe`,
    # drain will process any queued messages before removing interest.
    await sub.unsubscribe()
    await nc.drain()


if __name__ == '__main__':
    asyncio.run(main())
