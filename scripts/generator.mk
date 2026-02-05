#####################
##@ Generators            
#####################

# commands
CMD_NAME ?= newCommand

code_gen: ## generate code
	go generate

generate_cmd: build ## gernate new command
	./$(APP_NAME) generate cmd --name $(CMD_NAME)
