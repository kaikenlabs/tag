#####################
##@ Lints   
#####################

lint: ## Lint tools
	@echo "Running go vet"
	@go vet ./...
	@echo "Running gofumpt"
	@${GOBIN}/gofumpt -l -w .
	@echo "Running deadcode"
	@${GOBIN}/deadcode ./...
	@echo "Running golangci-lint"
	@${GOBIN}/golangci-lint -c .golangci.yaml run ./...

scan: ## Security scanning
	@${GOBIN}/gosec ./...
	@${GOBIN}/govulncheck ./...
