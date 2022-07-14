# NATS By Example

Collection of reference examples using NATS ranging from basic messaging to advanced architecture design and operational concerns.

There are three goals for this repo:

- Provide fully functional and robust reference examples for NATS
- Sufficiently document each example and make them presentable for learning
- Keep the examples up-to-date

**Note: this repo just started, so please be patient while examples are being added. See how you can help by [contributing](#contributing)!**

## Design

### Directory structure

Under the `examples` directory, each category has one or more examples with one or more implementations. For example:

```
examples/
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

### Implementation directory

The directory is named after the NATS client they correspond to, either the language name, e.g. `go` or the CLI, e.g. `cli`. For multi-client examples or ones requiring for complex setups, the directory can be named `shell` to indicate a custom shell script is being used.

The entrypoint file is expected to be named `main.[ext]` where the `ext` is language specific or `sh` for a shell script (including CLI usage). In addition to convention, the significance of this file is that the comments will be extracted out to be rendered more legibly alongside the source code.

Each implementation should also include a custom `Dockerfile` to be able to build and run the example in a container acting as a controlled, reproducible environment. If not provided, the tooling will attempt generate one based on the language.

Similiary, a `docker-compose.yaml` will be generated automatically so examples have a fresh NATS server to run against. Implementations should rely on the `NATS_URL` environment variable to when creating a client connection.

In both cases, the `Dockerfile` and `docker-compose.yaml` will have a comment at the top of file indicating if the file was generated and a timestamp. Only files with this comment will be regenerated in subsequent builds.

The final file of interest is the `output.txt` file is generated automatically from the output of running the program. The purpose of this file is for the convenience of viewing it alongside the code without needing to run the program.

### Command-line interface

TODO

## Contributing

There are several ways to contribute!

- Create an issue for an issue with an existing example (comment or code)
- Create an issue to for a new implementation of an existing example
- Create an issue to recommend a new example
- Create a pull request to fix an existing example
- Create a pull request for a new implementation of an existing example
- Create a pull request with an implementation of a new example
