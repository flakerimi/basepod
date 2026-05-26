package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func loginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to a BasePod server and save the resulting token",
		Long: `Prompts for the server URL, username, and password, then issues a long-lived
API token and writes it to ~/.basepod/config.yaml. Subsequent commands use the
saved token.`,
		Example: `  basepod login
  basepod login --server http://localhost:8080
  basepod login --server https://bp.example.com`,
		RunE: func(cmd *cobra.Command, args []string) error {
			serverFlag, _ := cmd.Flags().GetString("server")
			cfg, _ := loadConfig()
			if serverFlag != "" {
				cfg.Server = serverFlag
			}
			if cfg.Server == "" {
				cfg.Server = promptDefault("Server URL", "http://localhost:8080")
			}
			username := prompt("Username")
			password := promptPassword("Password")

			// 1. login (cookie)
			c := &Client{cfg: cfg, http: defaultHTTP()}
			var loginResp struct {
				User struct{ ID string } `json:"user"`
			}
			if err := c.JSON(context.Background(), "POST", "/api/v1/auth/login", map[string]string{
				"username": username,
				"password": password,
			}, &loginResp); err != nil {
				return err
			}
			// 2. issue token (using the session cookie from login)
			// Simpler: re-login via the same client which carries no cookies; we
			// instead persist credentials by issuing a token using basic auth-style:
			// the login response set a session, but our http client did not retain
			// the cookie. So do it the explicit way using the same Client + Cookie jar.
			return issueAndSaveToken(cfg, username, password)
		},
	}
	return cmd
}

func issueAndSaveToken(cfg Config, username, password string) error {
	c := &Client{cfg: cfg, http: defaultHTTPWithJar()}
	if err := c.JSON(context.Background(), "POST", "/api/v1/auth/login", map[string]string{
		"username": username,
		"password": password,
	}, nil); err != nil {
		return err
	}
	var tokenResp struct {
		Token string `json:"token"`
		Name  string `json:"name"`
	}
	if err := c.JSON(context.Background(), "POST", "/api/v1/auth/tokens", map[string]string{
		"name": "cli-" + hostname(),
	}, &tokenResp); err != nil {
		return err
	}
	cfg.Token = tokenResp.Token
	if err := saveConfig(cfg); err != nil {
		return err
	}
	fmt.Printf("logged in to %s as %s\n", cfg.Server, username)
	return nil
}

func logoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Clear local credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := loadConfig()
			cfg.Token = ""
			if err := saveConfig(cfg); err != nil {
				return err
			}
			fmt.Println("logged out")
			return nil
		},
	}
}

func tokensCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tokens",
		Short: "Manage API tokens",
		Example: `  basepod tokens list
  basepod tokens create ci
  basepod tokens revoke 9f4a...e2`,
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List API tokens",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			var out struct {
				Tokens []struct {
					ID         string `json:"id"`
					Name       string `json:"name"`
					CreatedAt  int64  `json:"created_at"`
					RevokedAt  *int64 `json:"revoked_at"`
				} `json:"tokens"`
			}
			if err := c.JSON(context.Background(), "GET", "/api/v1/auth/tokens", nil, &out); err != nil {
				return err
			}
			for _, t := range out.Tokens {
				status := "active"
				if t.RevokedAt != nil {
					status = "revoked"
				}
				fmt.Printf("%s  %s  %s\n", t.ID, t.Name, status)
			}
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "create <name>",
		Short: "Issue a new API token",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			var out struct {
				Token string `json:"token"`
				Name  string `json:"name"`
			}
			if err := c.JSON(context.Background(), "POST", "/api/v1/auth/tokens", map[string]string{"name": args[0]}, &out); err != nil {
				return err
			}
			fmt.Println(out.Token)
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "revoke <id>",
		Short: "Revoke an API token",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			return c.JSON(context.Background(), "DELETE", "/api/v1/auth/tokens/"+args[0], nil, nil)
		},
	})
	return cmd
}

// helpers

func hostname() string {
	h, _ := os.Hostname()
	if h == "" {
		return "host"
	}
	return h
}

func getServer(cmd *cobra.Command) string {
	v, _ := cmd.Flags().GetString("server")
	if v == "" && cmd.Root() != nil {
		v, _ = cmd.Root().PersistentFlags().GetString("server")
	}
	return v
}

func prompt(label string) string {
	fmt.Printf("%s: ", label)
	rd := bufio.NewReader(os.Stdin)
	line, _ := rd.ReadString('\n')
	return strings.TrimSpace(line)
}

func promptDefault(label, def string) string {
	fmt.Printf("%s [%s]: ", label, def)
	rd := bufio.NewReader(os.Stdin)
	line, _ := rd.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

func promptPassword(label string) string {
	fmt.Printf("%s: ", label)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
