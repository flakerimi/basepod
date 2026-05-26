package main

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

func appPortCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "port",
		Short: "Manage container ports",
		Example: `  basepod app port add my-api 3000
  basepod app port rm  my-api 3000`,
	}
	cmd.AddCommand(&cobra.Command{
		Use:  "add <app> <port>",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			var p int
			if _, err := fmt.Sscanf(args[1], "%d", &p); err != nil || p < 1 || p > 65535 {
				return fmt.Errorf("port must be 1..65535")
			}
			return c.JSON(context.Background(), "POST", "/api/v1/apps/"+args[0]+"/ports", map[string]any{"port": p}, nil)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:  "rm <app> <port>",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			return c.JSON(context.Background(), "DELETE", "/api/v1/apps/"+args[0]+"/ports/"+args[1], nil, nil)
		},
	})
	return cmd
}

func appVolumeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "volume",
		Short: "Manage volumes (bind-mounts / named)",
		Example: `  basepod app volume add my-api /data ~/BasePodData/my-api/data
  basepod app volume rm  my-api /data`,
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "add <app> <container_path> <host_path>",
		Short: "Add a bind-mount volume",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			return c.JSON(context.Background(), "POST", "/api/v1/apps/"+args[0]+"/volumes", map[string]any{
				"container": args[1],
				"host":      args[2],
			}, nil)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:  "rm <app> <container_path>",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			esc := url.PathEscape(strings.TrimPrefix(args[1], "/"))
			return c.JSON(context.Background(), "DELETE", "/api/v1/apps/"+args[0]+"/volumes/"+esc, nil, nil)
		},
	})
	return cmd
}
