COVER_OUTPUT_RAW := coverage.out
COVER_OUTPUT_HTML := coverage.html

#####################
##@ Tests
#####################

test: ## Run all tests with coverage
	@$(MISE) go test -coverprofile=$(COVER_OUTPUT_RAW) -failfast ./...

test-integration: ## Run the integration tests only
	@$(MISE) go test -count=1 -v ./internal/integration/...

test-cover: ## generate html coverage report
	@$(MISE) go tool cover -html=$(COVER_OUTPUT_RAW) -o $(COVER_OUTPUT_HTML)
	@echo "Coverage report generated: $(COVER_OUTPUT_HTML)"
