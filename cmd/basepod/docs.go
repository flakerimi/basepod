package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func docsCmd() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:    "docs <dir>",
		Short:  "Generate Markdown reference for every command",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir = args[0]
			if err := doc.GenMarkdownTree(cmd.Root(), dir); err != nil {
				return err
			}
			fmt.Println("wrote markdown docs to", dir)
			return nil
		},
	}
	return cmd
}
