# Neo4j Graph Tool Core

This repository contains the driver code for the Neo4j Graph Tool. This Tool Core offers the following features:

- Replaces the Neo4j Docker Image entrypoint.
- Exposes a HTTP server, enables basic operations that can be carried on the Neo4j database.
- Data migrations are executed with Cypher scripts, using the Cypher shell command-line interface.
- Semantic Versioning is used when versioning the scripts.
- Command ```pre-commit run -a``` runs all tests.
- Command ```golangci-lint run``` only runs test in the current folder.
