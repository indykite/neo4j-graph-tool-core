.PHONY: fmt goimports gci test cover lint upgrade download install-tools

fmt:
	@echo "==> Fixing source code with gofmt..."
	gofmt -s -w .

# Keep for backward compatibility
goimports: gci

gci:
	@echo "==> Fixing imports code with gci"
	gci write -s standard -s default -s "prefix(github.com/indykite/neo4j-graph-tool-core)" -s blank -s dot .

generate_mocks:
	cd test && go generate

test:
	go test -v -cpu 4 -covermode=count -coverpkg github.com/indykite/neo4j-graph-tool-core/... -coverprofile=coverage.out ./...

cover: test
	go tool cover -html=coverage.out

lint:
	golangci-lint run --timeout 2m0s ./...

upgrade:
	go get -u all && go mod tidy

install-tools: download
	@echo Installing tools
	@go install github.com/daixiang0/gci@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/golang/mock/mockgen@latest
	@go install github.com/onsi/ginkgo/v2/ginkgo@latest
	@echo Installation completed
