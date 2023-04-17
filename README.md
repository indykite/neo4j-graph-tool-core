# Neo4j Graph Tool Core

This repository contains the driver code for the [Neo4j Graph Tool](https://github.com/indykite/neo4j-graph-tool).
Refer to that boilerplate to build your own Graph Tool for development with Neo4j.

## Developing with Core

The Core offers the following features:

- Configuration with ENV vars or one of many file formats. We are using [Go Viper library](https://github.com/spf13/viper#why-viper).
- Replacement for the Neo4j Docker Image entrypoint.
- Exposes a HTTP server, enables basic operations that can be executed on the Neo4j instance running inside Docker container.
- Data migrations are executed with Cypher scripts, using the Cypher shell command-line interface.
- Semantic Versioning is used when versioning the scripts.

## Contribution

Just open PR and don't forget to run

- Command ```pre-commit run -a``` runs all tests. (Requires [pre-commit](https://pre-commit.com/#install) to be installed)
- Command ```make lint``` only runs test in the current folder.
- Command ```make test``` for test execution.

## Usage

For real usage look into [Neo4j Graph Tool](https://github.com/indykite/neo4j-graph-tool).

## Configuration

Configuration examples and the source code can be found in the [config folder](config).
Here is a description of some important functions and methods.

- **New()** creates a configuration structure with default values.
    A configuration structure is a structure of type Config that contains all the necessary values.

- **LoadFile(pathToFile string)** creates a configuration structure from values specified in a file.

The configuration loading is handled by the Go package [Viper](https://github.com/spf13/viper).
Viper supports multiple configuration formats like XML, JSON, YAML, TOML etc.
Additionally, it supports environment variables. It should be noted that environment variables have higher priority
than values defined in a config file.

Environment variable names must be defined in this format: ```GT_SUPERVISOR_PORT```.
*GT* is a prefix for all environment variables.
*SUPERVISOR_PORT* indicates that the field Port in Supervisor will be defined.

For more information check out the [config_test.go file](config/config_test.go) which contains tests.
The example files with values can be found in the folder [testdata](config/testdata/).
Note that test files are written using the TOML format.
