package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "basepod",
		Short: "BasePod CLI — manage services on the local BasePod server",
		Long: `BasePod CLI manages a BasePod server: deploy apps, attach domains,
install one-click templates, and manage credentials.

Configuration lives at ~/.basepod/config.yaml (server URL + API token, set by
'basepod login'). All commands accept --server to point at a different host.`,
		Example: `  # log in to a server
  basepod login --server http://localhost:8080

  # deploy from the current directory (tarballs the dir, ships it, builds, deploys)
  basepod app deploy myapp .

  # deploy a pre-built image
  basepod app deploy myapp --image ghcr.io/me/myapp:1.2.0

  # tail container logs
  basepod app logs myapp

  # spin up a postgres via template
  basepod template install postgres my-db POSTGRES_PASSWORD=secret

  # attach a custom domain
  basepod domain add myapp shop.example.com

  # back up server state + data
  basepod backup -o backup.tar.gz`,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.PersistentFlags().String("server", "", "BasePod server URL (defaults to ~/.basepod/config.yaml)")
	cmd.AddCommand(loginCmd())
	cmd.AddCommand(logoutCmd())
	cmd.AddCommand(tokensCmd())
	cmd.AddCommand(appCmd())
	cmd.AddCommand(domainCmd())
	cmd.AddCommand(templateCmd())
	cmd.AddCommand(settingsCmd())
	cmd.AddCommand(backupCmd())
	cmd.AddCommand(completionCmd())
	cmd.AddCommand(docsCmd())
	cmd.AddCommand(versionCmd())
	return cmd
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show CLI version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("basepod", version)
		},
	}
}
