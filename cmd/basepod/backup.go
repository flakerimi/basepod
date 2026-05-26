package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
)

func backupCmd() *cobra.Command {
	var out string
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Download a backup of server state (sqlite + data dir + caddy config)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient(getServer(cmd))
			if err != nil {
				return err
			}
			if out == "" {
				out = fmt.Sprintf("basepod-backup-%s.tar.gz", time.Now().UTC().Format("20060102-150405"))
			}
			req, err := http.NewRequest("POST", c.url("/api/v1/backup"), nil)
			if err != nil {
				return err
			}
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
			f, err := os.Create(out)
			if err != nil {
				return err
			}
			defer f.Close()
			n, err := io.Copy(f, resp.Body)
			if err != nil {
				return err
			}
			fmt.Printf("wrote %d bytes to %s\n", n, out)
			return nil
		},
	}
	cmd.Flags().StringVarP(&out, "out", "o", "", "Output path (default: basepod-backup-TIMESTAMP.tar.gz)")
	return cmd
}
