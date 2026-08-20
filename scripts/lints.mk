#####################
##@ Lints
#####################

GOSEC_EXCLUDE := G104,G122,G304,G703,G706

lint: ## Lint and check formatting
	@echo "Running go vet"
	@$(MISE) go vet ./...
	@echo "Running deadcode"
	@out=$$($(MISE) deadcode -test ./...) || { printf '%s\n' "$$out"; exit 1; }; \
		if [ -n "$$out" ]; then printf '%s\n' "$$out"; exit 1; fi
	@echo "Running golangci-lint"
	@$(MISE) golangci-lint -c .golangci.yaml run ./...

fmt: ## Format code with gofumpt and goimports
	@echo "Running gofumpt"
	@$(MISE) gofumpt -w .
	@echo "Running goimports"
	@$(MISE) goimports -w -local github.com/kaikenlabs/tag .

scan-code: ## Static security analysis (gosec) — gated in CI
	@$(MISE) gosec -quiet -exclude-dir=.claude -exclude=$(GOSEC_EXCLUDE) ./...

scan-deps: ## Known-vulnerability scan of dependencies (govulncheck) — advisory
	@$(MISE) govulncheck ./...

scan: scan-code scan-deps ## Security scanning
