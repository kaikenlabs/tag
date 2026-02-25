#####################
##@ Lints
#####################

lint: ## Lint and check formatting
	@echo "Running go vet"
	@go vet ./...
	@echo "Running deadcode"
	@${GOBIN}/deadcode ./...
	@echo "Running golangci-lint"
	@${GOBIN}/golangci-lint -c .golangci.yaml run ./...

fmt: ## Format code with gofumpt and goimports
	@echo "Running gofumpt"
	@${GOBIN}/gofumpt -w .
	@echo "Running goimports"
	@${GOBIN}/goimports -w -local github.com/kaikenlabs/tag .

scan: ## Security scanning
	@${GOBIN}/gosec ./...
	@${GOBIN}/govulncheck ./...
