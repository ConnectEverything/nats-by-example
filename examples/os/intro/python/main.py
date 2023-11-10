import io
import os
import asyncio

import nats
from nats.js.api import ObjectMeta
from nats.js.errors import BucketNotFoundError
from nats.js.object_store import ObjectStore

servers = os.environ.get('NATS_URL', 'nats://localhost:4222').split(',')
bucket_name = 'configs'


# Set up an async function that we'll use later
async def notify_on_update(watcher: ObjectStore.ObjectWatcher):
    while True:
        e = await watcher.updates()
        if e:
            update_type = 'deleted' if e.deleted else 'updated'
            print(f'{bucket_name} changed - {e.name} was {update_type}')


async def main():
    # Connect to NATS server
    nc = await nats.connect(servers=servers)

    # Get a context to produce and consume messages from NATS JetStream
    js = nc.jetstream()

    # Try to access a store called 'configs'.
    # Create the store if it doesn't already exist.
    try:
        object_store = await js.object_store(bucket_name)
    except BucketNotFoundError:
        object_store = await js.create_object_store(bucket_name)

    # You can get information on the object store by getting its info:
    status = await object_store.status()
    print(f'the object store has {status.size} bytes')

    # 10 MiB
    data = bytes(10000000)

    # The Python client has a simple API to `put()`
    # Let's add an entry to the object store
    info = await object_store.put('a', data)
    print(f'added entry {info.name} ({info.size} bytes)')

    # Let's add another one with custom metadata
    info = await object_store.put('b', data, meta=ObjectMeta(
        description='large data'
    ))
    print(f'added entry {info.name} ({info.size} bytes) "{info.description}"')

    # Entries in an object store are made from a 'metadata' that describes the object
    # And the payload. This allows you to store information about the significance of the
    # entry separate from the raw data.
    # You can update the metadata directly
    new_meta = info.meta
    new_meta.description = 'still large data'
    await object_store.update_meta('b', new_meta)

    # we expect this store to contain 2 entries
    # You can list its contents
    entries = await object_store.list()
    print(f'the object store contains {len(entries)} entries')

    # Now lets retrieve the item
    obr = await object_store.get('b')
    data = obr.data
    print(f'data has {len(data)} bytes')

    # If you are reading carefully, the payload we stored and read, is 10MiB in
    # size, but the server by default has a max payload of about 1MiB.
    # How can this be possible? - Ah that is what ObjectStore is about.
    # It automatically split into multiple messages and stored in the
    # stream. On read, it read multiple messages and put it together.
    print(f'client has a max payload of {nc.max_payload} bytes')

    # You can watch an object store for changes:
    watcher = await object_store.watch(include_history=False)
    loop = asyncio.get_event_loop()
    loop.create_task(notify_on_update(watcher))

    # To delete an entry:
    await object_store.delete('a')

    # Because the client may be working with large assets, ObjectStore
    # normally presents a 'Stream' based API. You can put data from anything
    # that inherits BufferedIOBase.
    buffered_data = io.BytesIO(data)
    info = await object_store.put('c', buffered_data, nats.js.api.ObjectMeta(
        description='set with a buffer'
    ))
    print(f'added entry {info.name} ({info.size} bytes)- "{info.description}"')

    # To read the entry:
    buffer = io.BytesIO()
    result = await object_store.get('c', buffer)
    # you can read the info on the object:
    print(f'{result.info.name} has {result.info.size} bytes')

    # to get the payload, read it from the buffer
    buffer.seek(0)
    chunk_size = 128 * 1024  # 128 KB in bytes
    while True:
        chunk = buffer.read(chunk_size)
        if not chunk:
            break
        print(f'read {len(chunk)} bytes')

    # You can delete the object store by simply calling `js.delete_object_store()`.
    # Note that after calling delete_object_store, the data is gone forever
    await js.delete_object_store(bucket_name)
    print('deleted object store')

    await nc.close()


if __name__ == '__main__':
    asyncio.run(main())
