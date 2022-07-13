# NATS By Example

> Collection of reference examples using NATS.

*See https://natsbyexample.com to start exploring.*

There are three goals of this repo:

- Provide fully functional and robust reference examples for NATS
- Sufficiently document each example and make them presentable for learning
- Keep the examples up-to-date

## Getting started

The recommended way to get started is to browser the [website](https://natsbyexample.com) which provides nicer navigation and presentation for the examples in this repo.

This repo comes with a command-line tool called `nbe` which can be downloaded from the [releases](./releases) page. It provides a command `nbe run` which can be used to build and run any example locally. To make this a seamless experience and support different client languages and runtimes, `nbe run` relies on a container image runtime (currently Docker) to build an image and run the container.

Requirements:

- Docker with Compose v2 built-in (e.g. `docker compose` is supported)
- The [`nbe`](./releases) CLI
- This repo cloned

Now you can navigate to the root of the repo and run an example:

```sh
$ nbe run messaging/pub-sub/cli
```

The example correspond to the directory structure under `examples/`, specifically `<category>/<example>/<client>`.

## Design

### Directory structure

Under the `examples` directory, each category has one or more examples with one or more clients. For example:

```
examples/
  meta.yaml
  messaging/
    meta.yaml
    pub-sub/
      meta.yaml
      cli/
        main.sh
      go/
        main.go
      python/
        main.py
```

### Meta files

The top-level `meta.yaml` is used to define the order of the categories.

```yaml
categories:
  - messaging
  - auth
  # etc...
```

The category `meta.yaml` supports the following properties:

```yaml
# Title of the category, defaults to a titlecase of the directory name.
title: string

# Description of the category.
description: string

# An ordered list of the example names within the category.
examples: [string]
```

The example `meta.yaml` supports the following properties:

```yaml
# Title of the example.
title: string

# Description of the example.
description: string
```

### Client directory

The directory is named after the NATS client they correspond to, either the language name, e.g. `go` or the CLI, e.g. `cli`. For multi-client examples or ones requiring for complex setups, the directory can be named `shell` to indicate a custom shell script is being used.

The entrypoint file is expected to be named `main.[ext]` where the `ext` is language specific or `sh` for a shell script (including CLI usage). In addition to convention, the significance of this file is that the comments will be extracted out to be rendered more legibly alongside the source code.

Each client may include a custom `Dockerfile` to be able to build and run the example in a container acting as a controlled, reproducible environment. If not provided, the default one, by language, in the [`docker/`](./docker) directory will be used.

Most examples require a NATS server, so there are two `docker-compose.yaml` files available to run the examples. These are located in the same directory.

In both cases, the `Dockerfile` and `docker-compose.yaml` will have a comment at the top of file indicating if the file was generated and a timestamp. Only files with this comment will be regenerated in subsequent builds.

The final file of interest is the `output.txt` file is generated automatically from the output of running the program. The purpose of this file is for the convenience of viewing it alongside the code without needing to run the program.

### Command-line interface

This repo comes with a CLI called `nbe`, which is primarily used for building and managing the examples themselves.

However, it has a convenience command called `run` which can any of the examples using Docker Compose. It handles uses the default `Dockerfile` and `docker-compose.yaml` if a custom one is not present for the client.

Simply reference the path to the example you want to run.

```
$ nbe run messaging/pub-sub/go
```

This currently requies Docker being installed and the version 2 of Compose (which is built-in to the `docker` CLI). By request, other container runtimes may be added (such as [Podman](https://podman.io/)).

## Contributing

There are several ways to contribute!

- Create an issue for an issue with an existing example (comment or code)
- Create an issue to for a new client of an existing example
- Create an issue to recommend a new example
- Create a pull request to fix an existing example
- Create a pull request for a new client of an existing example
