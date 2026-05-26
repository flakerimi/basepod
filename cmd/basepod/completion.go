package main

import (
	"os"

	"github.com/spf13/cobra"
)

func completionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "completion",
		Short:                 "Generate shell completion scripts",
		DisableFlagsInUseLine: true,
		Long: `Print a completion script for your shell. Examples:

  basepod completion bash > /usr/local/etc/bash_completion.d/basepod
  basepod completion zsh  > "${fpath[1]}/_basepod"
  basepod completion fish > ~/.config/fish/completions/basepod.fish

For one-shot use in the current shell:
  source <(basepod completion zsh)
`,
	}
	cmd.AddCommand(&cobra.Command{
		Use: "bash",
		RunE: func(c *cobra.Command, args []string) error {
			return c.Root().GenBashCompletion(os.Stdout)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use: "zsh",
		RunE: func(c *cobra.Command, args []string) error {
			return c.Root().GenZshCompletion(os.Stdout)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use: "fish",
		RunE: func(c *cobra.Command, args []string) error {
			return c.Root().GenFishCompletion(os.Stdout, true)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use: "powershell",
		RunE: func(c *cobra.Command, args []string) error {
			return c.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		},
	})
	return cmd
}
