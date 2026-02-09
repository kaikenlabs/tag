
# Define tools with their import paths
define TOOLS
    mockgen:github.com/golang/mock/mockgen
    gojq:github.com/itchyny/gojq/cmd/gojq
    golangci-lint:github.com/golangci/golangci-lint/v2/cmd/golangci-lint
    gofumpt:mvdan.cc/gofumpt
    gotest:github.com/rakyll/gotest
    godepgraph:github.com/kisielk/godepgraph
    gosec:github.com/securego/gosec/v2/cmd/gosec
    govulncheck:golang.org/x/vuln/cmd/govulncheck
    deadcode:golang.org/x/tools/cmd/deadcode
endef

# Convert the TOOLS definition into variables
$(foreach tool,$(shell echo '$(TOOLS)' | tr ' ' '\n' | grep .:),\
    $(eval INSTALL_$(word 1,$(subst :, ,$(tool))) := go install $(word 2,$(subst :, ,$(tool)))@latest)\
    $(eval $(word 1,$(subst :, ,$(tool))) := ${GOBIN}/$(lastword $(subst /, ,$(word 2,$(subst :, ,$(tool)))))))


tools: ## Get tools used by the project
	@$(foreach tool,$(shell echo '$(TOOLS)' | tr ' ' '\n' | grep .:),\
		$(INSTALL_$(word 1,$(subst :, ,$(tool)))) && ) true
