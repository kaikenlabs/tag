COVER_OUTPUT_RAW := coverage.out
COVER_OUTPUT_HTML := coverage.html

#####################
##@ Tests            
#####################

test: test-unit test-integration ## Run all tests

test-unit: ## Run unit tests
	@${GOBIN}/gotest -coverprofile $(COVER_OUTPUT_RAW) --short -cover  -failfast ./...

test-integration: build ## Run integration test
	ENV=DEV ./tag -p example/_templates generate fakr --meta foo=bar,bin=baz,enum_list=a-long-list

test-cover: ## generate html coverage report + open
	@go tool cover -html=$(COVER_OUTPUT_RAW) -o $(COVER_OUTPUT_HTML)
	@open coverage.html