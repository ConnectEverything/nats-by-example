import asyncio
import os
import random

import nats


async def main():
    # Use the NATS_URL env variable if defined, otherwise fallback to the default.
    nats_url = os.getenv("NATS_URL", "nats://localhost:4222")

    client = await nats.connect(nats_url)

    # `subscription.messages` returns an asynchronous iterator and allows us to not
    # wait for time-consuming operation and receive next message immediately.
    messages = (await client.subscribe("greet.*", max_msgs=50)).messages

    # Publish set of messages, each with order identifier.
    for i in range(50):
        await client.publish("greet.joe", f"hello {i}".encode())

    # Iterate over messages concurrently.
    # `25` is a limit for concurrent operations.
    semaphore = asyncio.Semaphore(25)

    async def process_message(message):
        async with semaphore:
            await asyncio.sleep(random.uniform(0, 0.5))
            print(f"received message: {message.data.decode()!r}")

    await asyncio.gather(*[process_message(msg) async for msg in messages])


if __name__ == "__main__":
    asyncio.run(main())
