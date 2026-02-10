package commands

import (
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/kaikenlabs/tag/pkg/app"
)

// Bash completion script adapted from urfave/cli's autocomplete/bash_autocomplete
// with PROG hardcoded to "tag".
const bashCompletionScript = `#! /bin/bash

_cli_init_completion() {
  COMPREPLY=()
  _get_comp_words_by_ref "$@" cur prev words cword
}

_cli_bash_autocomplete() {
  if [[ "${COMP_WORDS[0]}" != "source" ]]; then
    local cur opts base words
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    if declare -F _init_completion >/dev/null 2>&1; then
      _init_completion -n "=:" || return
    else
      _cli_init_completion -n "=:" || return
    fi
    words=("${words[@]:0:$cword}")
    if [[ "$cur" == "-"* ]]; then
      requestComp="${words[*]} ${cur} --generate-bash-completion"
    else
      requestComp="${words[*]} --generate-bash-completion"
    fi
    opts=$(eval "${requestComp}" 2>/dev/null)
    COMPREPLY=($(compgen -W "${opts}" -- ${cur}))
    return 0
  fi
}

complete -o bashdefault -o default -o nospace -F _cli_bash_autocomplete tag
`

// Zsh completion script adapted from urfave/cli's autocomplete/zsh_autocomplete
// with PROG hardcoded to "tag".
const zshCompletionScript = `#compdef tag

_cli_zsh_autocomplete() {
  local -a opts
  local cur
  cur=${words[-1]}
  if [[ "$cur" == "-"* ]]; then
    opts=("${(@f)$(${words[@]:0:#words[@]-1} ${cur} --generate-bash-completion)}")
  else
    opts=("${(@f)$(${words[@]:0:#words[@]-1} --generate-bash-completion)}")
  fi

  if [[ "${opts[1]}" != "" ]]; then
    _describe 'values' opts
  else
    _files
  fi
}

compdef _cli_zsh_autocomplete tag
`

// CompletionCommand returns a command that outputs shell completion scripts.
// It requires a reference to the *cli.App for fish completion generation.
func CompletionCommand(a *cli.App) *cli.Command {
	return &cli.Command{
		Name:  "completion",
		Usage: "Output shell completion script",
		Description: `Output a shell completion script for bash, zsh, or fish.

SETUP:

  # Bash (add to ~/.bashrc)
  source <(tag completion bash)

  # Zsh (add to ~/.zshrc)
  source <(tag completion zsh)

  # Fish
  tag completion fish | source`,
		Subcommands: []*cli.Command{
			{
				Name:  "bash",
				Usage: "Output bash completion script",
				Action: func(c *cli.Context) error {
					fmt.Print(bashCompletionScript)
					return nil
				},
			},
			{
				Name:  "zsh",
				Usage: "Output zsh completion script",
				Action: func(c *cli.Context) error {
					fmt.Print(zshCompletionScript)
					return nil
				},
			},
			{
				Name:  "fish",
				Usage: "Output fish completion script",
				Action: func(c *cli.Context) error {
					fish, err := a.ToFishCompletion()
					if err != nil {
						return app.Errorf("failed to generate fish completion: %w", err)
					}
					fmt.Print(fish)
					return nil
				},
			},
		},
	}
}
