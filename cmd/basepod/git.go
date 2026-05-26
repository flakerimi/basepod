package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

// Registers `basepod app git` subcommands. Attached from app.go via cobra.AddCommand.
func appGitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "git",
		Short: "Configure git deploys + webhooks (GitHub, GitLab, Bitbucket)",
		Example: `  basepod app git show my-api
  basepod app git set my-api --url https://github.com/me/repo --branch main --token ghp_xxx
  basepod app git set my-api --url https://gitlab.com/me/repo --user me --pass GITLAB_PAT
  basepod app git webhook my-api      # generate / rotate the webhook secret
  basepod app git deploy my-api       # build from stored git config (CapRover "force build")`,
	}
	cmd.AddCommand(appGitShowCmd(), appGitSetCmd(), appGitWebhookCmd(), appGitDeployCmd())
	return cmd
}

func appGitShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:  "show <app>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			var out map[string]any
			if err := c.JSON(context.Background(), "GET", "/api/v1/apps/"+args[0]+"/git", nil, &out); err != nil {
				return err
			}
			for k, v := range out {
				printlnf("%-15s %v", k, v)
			}
			return nil
		},
	}
}

func appGitSetCmd() *cobra.Command {
	var url, branch, dockerfile, user, pass, token string
	cmd := &cobra.Command{
		Use:  "set <app>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			body := map[string]any{
				"url":        url,
				"branch":     branch,
				"dockerfile": dockerfile,
			}
			if token != "" {
				body["token"] = token
			}
			if user != "" {
				body["username"] = user
			}
			if pass != "" {
				body["password"] = pass
			}
			return c.JSON(context.Background(), "PUT", "/api/v1/apps/"+args[0]+"/git", body, nil)
		},
	}
	cmd.Flags().StringVar(&url, "url", "", "Git URL (https://github.com/owner/repo[.git])")
	cmd.Flags().StringVar(&branch, "branch", "main", "Branch to deploy")
	cmd.Flags().StringVar(&dockerfile, "dockerfile", "Dockerfile", "Path to Dockerfile inside the repo")
	cmd.Flags().StringVar(&token, "token", "", "GitHub PAT or generic token (preferred over user/pass)")
	cmd.Flags().StringVar(&user, "user", "", "Username (GitLab/Bitbucket-style auth)")
	cmd.Flags().StringVar(&pass, "pass", "", "Password / app password to pair with --user")
	return cmd
}

func appGitWebhookCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "webhook <app>",
		Short: "Generate or rotate the webhook secret + print the webhook URL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			var out struct {
				Secret     string `json:"secret"`
				WebhookURL string `json:"webhook_url"`
			}
			if err := c.JSON(context.Background(), "POST", "/api/v1/apps/"+args[0]+"/webhook-secret", nil, &out); err != nil {
				return err
			}
			fmt.Println("webhook URL:", out.WebhookURL)
			fmt.Println("secret:     ", out.Secret)
			fmt.Println()
			fmt.Println("Add to GitHub: Settings -> Webhooks -> Add webhook")
			fmt.Println("  Payload URL:  " + out.WebhookURL)
			fmt.Println("  Content type: application/json")
			fmt.Println("  Secret:       (paste the value above)")
			fmt.Println("  Events:       Just the push event")
			return nil
		},
	}
}

func appGitDeployCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "deploy <app>",
		Short: `Deploy from the stored git config (CapRover "force build")`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			return c.JSON(context.Background(), "POST", "/api/v1/apps/"+args[0]+"/deploy", map[string]any{"from_stored": true}, nil)
		},
	}
}
