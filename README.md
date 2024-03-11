# NATS By Example

> A collection of reference examples using NATS.

See https://natsbyexample.com to start exploring. Check out the [6m intro video](https://www.youtube.com/watch?v=GGX0KQuY0zQ) to learn how to best use this resource.

## Motivation

The vast majority of code examples that exist today don't work due to being incomplete or having invalid syntax **OR** the person trying to run the example doesn't know to properly setup the environment and/or dependencies for the example to run properly.

There are three primary goals of this repo:

- Provide fully functional and robust reference examples for as many NATS
- Sufficiently document each example and make them presentable for learning
- Keep the examples up-to-date

## Getting started

The recommended way to get started is to browse the [website](https://natsbyexample.com) which provides nicer navigation and presentation for the example code in this repo.

When you want to actually execute the example code, you need:

1. Clone this repository.
2. Download the [nbe](https://github.com/ConnectEverything/nats-by-example/releases) CLI, extract binary to the root of the cloned repository.
Currently, the nbe CLI needs [Docker](https://docs.docker.com/) and [Compose](https://docs.docker.com/compose/) (v2+) to work. It runs a set of containers hosting the CLI client and the NATS server. Other container runtimes would be considered if requested (such as [Podman](https://podman.io/)).
4. Install [Docker](https://docs.docker.com/) and [Compose](https://docs.docker.com/compose/) (if you do not have them installed).
Make sure that docker is installed and up, with command:
```sh
$ sudo systemctl start docker
```
5. Run the command with an example you want to try at the root of the repo:
```sh
$ nbe run messaging/pub-sub/cli
```

This will run the NATS CLI implementation of the [core publish-subscribe example](https://natsbyexample.com/examples/messaging/pub-sub/cli/) in a set of containers.

If everything is ok, you will see this output in console (timestamp will be different):
```sh
09:17:59 Published 5 bytes to "greet.joe" 
09:17:59 Subscribing on greet.* 
09:18:00 Published 5 bytes to "greet.joe"

[1] Received on "greet.joe" 
hello

[2] Received on "greet.pam"
```
The name of the example corresponds to the directory structure under `examples/`, specifically `<category>/<example>/<client>`.

Have questions, issues, or suggestions? Please open [start a discussion](https://github.com/ConnectEverything/nats-by-example/discussions) or open [an issue](https://github.com/ConnectEverything/nats-by-example/issues).

## Design

### Directory structure

Under the `examples` directory, each category has one or more examples with one or more client implementations. For example:

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
# Ordered set of categories.
categories: [string]
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
# Title of the example, defaults to the title-case of the directory name.
title: string

# Description of the example.
description: string
```

### Client directory

The directory is named after the NATS client they correspond to, either the language name, e.g. `go` or the CLI, e.g. `cli`. For multi-client examples or ones requiring complex setups, the directory can be named `shell` to indicate a custom shell script is being used.

The entrypoint file is expected to be named `main.[ext]` where the `ext` is language specific or `sh` for a shell script (including CLI usage). In addition to convention, the significance of this file is that the comments will be extracted out to be rendered more legibly alongside the source code for the website.

Each client may include a custom `Dockerfile` to be able to build and run the example in a container acting as a controlled, reproducible environment. If not provided, the default one, by language, in the [`docker/`](./docker) directory will be used.

Most examples require a NATS server, so there are two `docker-compose.yaml` files available in `docker/` which will be used by default. If there is a need for a customer file for an example, it can be added to the example directory to override the default.

## Contributing

There are several ways to contribute!

- Create an issue for an issue with an existing example (comment or code)
- Create an issue to for a new client of an existing example
- Create an issue to recommend a new example
- Create a pull request to fix an existing example
- Create a pull request for a new client of an existing example
