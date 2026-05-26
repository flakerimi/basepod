package main

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func appCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "app",
		Short: "Manage apps (create, deploy, scale, env, domains, logs, rollback)",
		Example: `  basepod app list
  basepod app create my-api --image ghcr.io/me/api:1 --port 3000
  basepod app deploy my-api .                       # tarball current dir, build, deploy
  basepod app deploy my-api --image ghcr.io/me/api:2
  basepod app env set my-api DATABASE_URL=postgres://...
  basepod app logs my-api
  basepod app versions my-api
  basepod app rollback my-api 20260525-203315-abcd1234
  basepod app update my-api --strategy stop_start --memory-mb 512
  basepod app scale my-api --instances 2
  basepod app destroy my-api`,
	}
	cmd.AddCommand(appListCmd(), appCreateCmd(), appShowCmd(), appDeployCmd(), appLogsCmd(), appRestartCmd(), appEnvCmd(), appDestroyCmd(), appVersionsCmd(), appRollbackCmd(), appScaleCmd(), appUpdateCmd(), appGitCmd(), appPortCmd(), appVolumeCmd())
	return cmd
}

func appRollbackCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rollback <name> <version>",
		Short: "Re-deploy a previous version",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			return c.JSON(context.Background(), "POST", "/api/v1/apps/"+args[0]+"/rollback", map[string]string{"version": args[1]}, nil)
		},
	}
}

func appScaleCmd() *cobra.Command {
	var instances int
	cmd := &cobra.Command{
		Use:   "scale <name> --instances N",
		Short: "Update the desired instance count (stored only in v1)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			return c.JSON(context.Background(), "PATCH", "/api/v1/apps/"+args[0], map[string]any{"instances": instances}, nil)
		},
	}
	cmd.Flags().IntVar(&instances, "instances", 1, "Instance count")
	return cmd
}

func appUpdateCmd() *cobra.Command {
	var strategy string
	var healthPath string
	var memoryMB, cpuPct int
	var internalOnly bool
	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Patch app config (strategy, healthcheck, limits, internal-only)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			body := map[string]any{}
			if cmd.Flags().Changed("strategy") {
				body["deploy_strategy"] = strategy
			}
			if cmd.Flags().Changed("health-path") {
				body["healthcheck_path"] = healthPath
			}
			if cmd.Flags().Changed("memory-mb") {
				body["memory_mb"] = memoryMB
			}
			if cmd.Flags().Changed("cpu-pct") {
				body["cpu_pct"] = cpuPct
			}
			if cmd.Flags().Changed("internal-only") {
				body["internal_only"] = internalOnly
			}
			if len(body) == 0 {
				return fmt.Errorf("nothing to update; pass at least one flag")
			}
			return c.JSON(context.Background(), "PATCH", "/api/v1/apps/"+args[0], body, nil)
		},
	}
	cmd.Flags().StringVar(&strategy, "strategy", "blue_green", "blue_green | stop_start")
	cmd.Flags().StringVar(&healthPath, "health-path", "", "HTTP healthcheck path")
	cmd.Flags().IntVar(&memoryMB, "memory-mb", 0, "Memory limit in MB (0 = unlimited)")
	cmd.Flags().IntVar(&cpuPct, "cpu-pct", 0, "CPU percent (0 = unlimited)")
	cmd.Flags().BoolVar(&internalOnly, "internal-only", false, "Exclude from Caddy routing")
	return cmd
}

func appListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List apps",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			var out struct {
				Apps []map[string]any `json:"apps"`
			}
			if err := c.JSON(context.Background(), "GET", "/api/v1/apps", nil, &out); err != nil {
				return err
			}
			fmt.Printf("%-20s  %-15s  %-30s\n", "NAME", "VERSION", "IMAGE")
			for _, a := range out.Apps {
				fmt.Printf("%-20s  %-15s  %-30s\n", a["name"], a["current_version"], a["image_repo"])
			}
			return nil
		},
	}
}

func appCreateCmd() *cobra.Command {
	var image string
	var ports []int
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create an app without deploying",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			body := map[string]any{"name": args[0]}
			if image != "" {
				body["image_repo"] = image
			}
			if len(ports) > 0 {
				body["ports"] = ports
			}
			var out map[string]any
			if err := c.JSON(context.Background(), "POST", "/api/v1/apps", body, &out); err != nil {
				return err
			}
			fmt.Println("created", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&image, "image", "", "Optional image reference")
	cmd.Flags().IntSliceVar(&ports, "port", nil, "Container port (repeatable)")
	return cmd
}

func appShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show app details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			var out struct {
				App map[string]any `json:"app"`
			}
			if err := c.JSON(context.Background(), "GET", "/api/v1/apps/"+args[0], nil, &out); err != nil {
				return err
			}
			b, _ := json.MarshalIndent(out.App, "", "  ")
			fmt.Println(string(b))
			return nil
		},
	}
}

func appDeployCmd() *cobra.Command {
	var image string
	var follow bool
	cmd := &cobra.Command{
		Use:   "deploy <app> [path]",
		Short: "Deploy an app from a directory tarball or image",
		Long: `Deploy <app> from a build context (directory or tarball path) or a pre-built
image. The server runs 'podman build' on the context, tags as basepod/<app>:<sha>,
then performs a blue/green or stop_start swap depending on the app's strategy.

Use --image to skip the build step and deploy an existing OCI image.
Use --follow to stream SSE events (build logs + deploy state) after queuing.`,
		Example: `  basepod app deploy my-api .
  basepod app deploy my-api ./build.tar.gz
  basepod app deploy my-api --image ghcr.io/me/api:1.2 --follow`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			name := args[0]
			if image != "" {
				return c.JSON(context.Background(), "POST", "/api/v1/apps/"+name+"/deploy", map[string]string{"image": image}, nil)
			}
			if len(args) < 2 {
				return fmt.Errorf("need a path or --image")
			}
			path := args[1]
			tarBuf, err := tarPath(path)
			if err != nil {
				return err
			}
			body := &bytes.Buffer{}
			mw := multipart.NewWriter(body)
			fw, _ := mw.CreateFormFile("tar", "context.tar")
			if _, err := io.Copy(fw, tarBuf); err != nil {
				return err
			}
			mw.Close()
			req, err := http.NewRequest("POST", c.url("/api/v1/apps/"+name+"/deploy"), body)
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", mw.FormDataContentType())
			if c.cfg.Token != "" {
				req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
			}
			resp, err := c.http.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode >= 400 {
				b, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("HTTP %d: %s", resp.StatusCode, b)
			}
			fmt.Println("deploy queued")
			if follow {
				return c.Stream(context.Background(), "/api/v1/events", func(s string) {
					fmt.Println(s)
				})
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&image, "image", "", "Deploy from a pre-built image instead of tarball")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Stream events after queuing the deploy")
	return cmd
}

func appLogsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logs <name>",
		Short: "Tail container logs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			return c.Stream(context.Background(), "/api/v1/apps/"+args[0]+"/logs", func(line string) {
				fmt.Println(line)
			})
		},
	}
}

func appRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart <name>",
		Short: "Restart the running container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			return c.JSON(context.Background(), "POST", "/api/v1/apps/"+args[0]+"/restart", nil, nil)
		},
	}
}

func appDestroyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "destroy <name>",
		Short: "Delete an app (does not remove data dirs)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			return c.JSON(context.Background(), "DELETE", "/api/v1/apps/"+args[0], nil, nil)
		},
	}
}

func appVersionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "versions <name>",
		Short: "List deployed versions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			var out struct {
				Versions []map[string]any `json:"versions"`
			}
			if err := c.JSON(context.Background(), "GET", "/api/v1/apps/"+args[0]+"/versions", nil, &out); err != nil {
				return err
			}
			fmt.Printf("%-25s  %-30s  %s\n", "VERSION", "IMAGE", "STATUS")
			for _, v := range out.Versions {
				fmt.Printf("%-25s  %-30s  %s\n", v["version"], v["image_tag"], v["status"])
			}
			return nil
		},
	}
}

func appEnvCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Manage environment variables",
		Example: `  basepod app env list my-api
  basepod app env set   my-api NODE_ENV=production DATABASE_URL=postgres://...
  basepod app env unset my-api OLD_VAR ANOTHER_VAR`,
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list <name>",
		Args:  cobra.ExactArgs(1),
		Short: "Show env",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			var out struct {
				Env map[string]string `json:"env"`
			}
			if err := c.JSON(context.Background(), "GET", "/api/v1/apps/"+args[0]+"/env", nil, &out); err != nil {
				return err
			}
			for k, v := range out.Env {
				fmt.Printf("%s=%s\n", k, v)
			}
			return nil
		},
	})
	var setRestart bool
	setCmd := &cobra.Command{
		Use:   "set <name> KEY=VAL [KEY=VAL...]",
		Short: "Set/merge env vars (preserves existing ones not listed)",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			// merge with existing so users don't have to re-send the full set
			var current struct {
				Env map[string]string `json:"env"`
			}
			if err := c.JSON(context.Background(), "GET", "/api/v1/apps/"+args[0]+"/env", nil, &current); err != nil {
				return err
			}
			if current.Env == nil {
				current.Env = map[string]string{}
			}
			for _, kv := range args[1:] {
				parts := strings.SplitN(kv, "=", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid pair: %s", kv)
				}
				current.Env[parts[0]] = parts[1]
			}
			path := "/api/v1/apps/" + args[0] + "/env"
			if setRestart {
				path += "?restart=1"
			}
			return c.JSON(context.Background(), "PUT", path, map[string]any{"env": current.Env}, nil)
		},
	}
	setCmd.Flags().BoolVar(&setRestart, "restart", false, "Restart the container after saving")
	cmd.AddCommand(setCmd)

	cmd.AddCommand(&cobra.Command{
		Use:   "unset <name> KEY [KEY...]",
		Short: "Delete one or more env vars",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			for _, k := range args[1:] {
				if err := c.JSON(context.Background(), "DELETE", "/api/v1/apps/"+args[0]+"/env/"+k, nil, nil); err != nil {
					return err
				}
			}
			return nil
		},
	})
	return cmd
}

func tarPath(p string) (io.Reader, error) {
	info, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return os.Open(p) // assume tarball
	}
	buf := &bytes.Buffer{}
	tw := tar.NewWriter(buf)
	err = filepath.Walk(p, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(p, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if isIgnored(rel) {
			if fi.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		hdr, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}
		hdr.Name = rel
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if !fi.Mode().IsRegular() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
	if err != nil {
		return nil, err
	}
	tw.Close()
	return buf, nil
}

func isIgnored(rel string) bool {
	for _, p := range []string{".git", "node_modules", ".DS_Store", "dist", "build"} {
		if rel == p || strings.HasPrefix(rel, p+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
