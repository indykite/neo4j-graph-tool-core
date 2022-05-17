.PHONY: test cover lint goimports

goimports:
	@echo "==> Fixing imports code with goimports"
	goimports -local "github.com/indykite/neo4j-graph-tool-core" -w ./...

test:
	go test -v -cpu 4 -covermode=count -coverpkg github.com/indykite/neo4j-graph-tool-core/... -coverprofile=coverage.out ./...

cover: test
	go tool cover -html=coverage.out

lint:
	golangci-lint run --timeout 2m0s ./...
