# Neo4j Graph Tool Core

This repository contains the driver code for the [Neo4j Graph Tool](https://github.com/indykite/neo4j-graph-tool).
Refer to that boilerplate to build your own Graph Tool for development with Neo4j.

## Developing with Core

The Core offers the following features:

- Configuration with ENV vars or one of many file formats. We are using [Go Viper library](https://github.com/spf13/viper#why-viper).
- Replacement for the Neo4j Docker Image entrypoint.
- Exposes a HTTP server, enables basic operations that can be executed on the Neo4j instance running inside Docker container.
- Data migrations are executed with Cypher scripts, using the *Cypher shell* command-line interface.
- Semantic Versioning is used when versioning the scripts.

## Contribution

Just open PR and don't forget to run

- Command ```pre-commit run -a``` runs all tests. (Requires [pre-commit](https://pre-commit.com/#install) to be installed)
  - Ruby is required for `markdownlint`. You can use [ASDF](https://asdf-vm.com/)
- Command ```make lint``` only runs linters for current project.
- Command ```make test``` for test execution.

## Usage

For real usage look into [Neo4j Graph Tool](https://github.com/indykite/neo4j-graph-tool).

## Documentation

This is just framework, which can be used as a package to integrate into your code.
Example integration is in [Neo4j Graph Tool](https://github.com/indykite/neo4j-graph-tool),
which can be forked and used directly, rather than this package.

Also usage documentation is described there. Here are just described main parts, which you can use, when building own

### Configuration

Configuration examples and the source code can be found in the [config folder](config).

The configuration loading is handled by the Go package [Viper](https://github.com/spf13/viper).
Viper supports multiple configuration formats like XML, JSON, YAML, TOML etc.
Additionally, it supports environment variables. It should be noted that environment variables have higher priority
than values defined in a config file.

For more information check out the [config_test.go file](config/config_test.go) which contains tests.
The example files with values can be found in the folder [testdata](config/testdata/).
Note that test files are written using the TOML format.

### Migrator

Migrator is responsible for planning which cyphers or files will be executed. It consists of scanner and planner.

**Scanner** scans all folders based on passed config and validate the naming and versions.
It also reach DB server and fetch which files were already executed.

**Planner** takes the output of scanner and based on target version prepare a plan, which files should be executed.
And if asked, start the migrating process by executing the plan.

#### Another migrator features

Migrator supports feature we call **Batch**es. There always must be main folder, with default name `schema`.
But it is possible to add additional folders. Then you can specify batch, which consist of multiple folders.
This can be useful for seeding, when different data should be seeded for different environments.

Migrator supports running migration with **Cypher** or by **executing binary**.
With `*.cypher` files, which is executed with Cypher shell, or `*.run` file, which can execute any command.
This is useful, when some migration cannot be accomplished with pure Cypher.
However, only white-listed commands in config can be executed with `*.run` file.

Very useful could be also **Snapshots**, which could speed up running all migrations on clear DB.
You can create snapshot per each version and batch.

### Supervisor

Supervisor is replacement for Docker image entrypoint and manage Neo4j instance by itself.

Supervisor also expose a simple HTTP server, which can be used to trigger some actions with Neo4j instance.
Either directly start/stop/restart Neo4j server, run all migrations or delete all data and then run migrations etc.

Supervisor has its own section in configuration, which can be ignored, if supervisor will not be used.
