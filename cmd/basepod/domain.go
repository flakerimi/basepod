package main

import (
	"context"

	"github.com/spf13/cobra"
)

func domainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "domain",
		Short: "Manage app domains",
		Example: `  basepod domain list my-api
  basepod domain add  my-api shop.example.com
  basepod domain rm   my-api shop.example.com`,
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list <app>",
		Args:  cobra.ExactArgs(1),
		Short: "List domains attached to an app",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			var out struct {
				App struct {
					Domains []struct {
						Domain    string `json:"domain"`
						IsPrimary bool   `json:"is_primary"`
						TLSState  string `json:"tls_state"`
					} `json:"domains"`
				} `json:"app"`
			}
			if err := c.JSON(context.Background(), "GET", "/api/v1/apps/"+args[0], nil, &out); err != nil {
				return err
			}
			for _, d := range out.App.Domains {
				flag := ""
				if d.IsPrimary {
					flag = " (primary)"
				}
				printlnf("%s  [%s]%s", d.Domain, d.TLSState, flag)
			}
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "add <app> <fqdn>",
		Args:  cobra.ExactArgs(2),
		Short: "Attach a domain to an app",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			return c.JSON(context.Background(), "POST", "/api/v1/apps/"+args[0]+"/domains", map[string]string{"domain": args[1]}, nil)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "rm <app> <fqdn>",
		Args:  cobra.ExactArgs(2),
		Short: "Detach a domain from an app",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			return c.JSON(context.Background(), "DELETE", "/api/v1/apps/"+args[0]+"/domains/"+args[1], nil, nil)
		},
	})
	return cmd
}

func templateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Browse and install one-click apps (databases, caches, etc.)",
		Example: `  basepod template list
  basepod template install postgres my-db POSTGRES_PASSWORD=secret
  basepod template install redis cache`,
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List available templates",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			var out struct {
				Templates []map[string]any `json:"templates"`
			}
			if err := c.JSON(context.Background(), "GET", "/api/v1/templates", nil, &out); err != nil {
				return err
			}
			for _, t := range out.Templates {
				printlnf("%-15s  %-10s  %s", t["id"], t["version"], t["description"])
			}
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "install <id> <app-name>",
		Short: "Install a template as a new app",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			fields := map[string]string{}
			rest := args[2:]
			for _, kv := range rest {
				parts := splitKV(kv)
				if len(parts) == 2 {
					fields[parts[0]] = parts[1]
				}
			}
			return c.JSON(context.Background(), "POST", "/api/v1/templates/install", map[string]any{
				"template_id": args[0],
				"app_name":    args[1],
				"fields":      fields,
			}, nil)
		},
	})
	return cmd
}

func settingsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "settings",
		Short: "Get/set server settings (root_domain, acme_email, admin_subdomain, dns_*)",
		Example: `  basepod settings list
  basepod settings set root_domain=example.com acme_email=ops@example.com
  basepod settings set admin_subdomain=bp
  basepod settings set dns_provider=cloudflare dns_token=xxxx     # enables wildcard certs`,
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "Show settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			var out struct {
				Settings map[string]string `json:"settings"`
			}
			if err := c.JSON(context.Background(), "GET", "/api/v1/settings", nil, &out); err != nil {
				return err
			}
			for k, v := range out.Settings {
				printlnf("%s=%s", k, v)
			}
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "set KEY=VAL [KEY=VAL...]",
		Short: "Update one or more settings",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			payload := map[string]string{}
			for _, kv := range args {
				parts := splitKV(kv)
				if len(parts) == 2 {
					payload[parts[0]] = parts[1]
				}
			}
			return c.JSON(context.Background(), "PUT", "/api/v1/settings", payload, nil)
		},
	})
	return cmd
}

func splitKV(s string) []string {
	for i, r := range s {
		if r == '=' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}

func printlnf(format string, args ...any) {
	_ = printfImpl(format, args...)
}
