package cmd

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"github.com/okyashgajjar/costwise-mcp/internal/mcpserver"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the CostWise MCP Server",
	RunE: func(cmd *cobra.Command, args []string) error {
		httpAddr, _ := cmd.Flags().GetString("http")
		allowPaths, _ := cmd.Flags().GetStringArray("allow-path")

		if len(allowPaths) == 0 {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("cannot determine home directory: %w", err)
			}
			allowPaths = []string{home}
		}

		mcpserver.Init(allowPaths)
		mcpServer := mcpserver.NewServer()
		defer mcpserver.CloseAllSessions()

		if httpAddr != "" {
			fmt.Fprintf(os.Stderr, "HTTP transport is under development. Starting with Stdio instead.\n")
		}

		return server.ServeStdio(mcpServer)
	},
}

func init() {
	serveCmd.Flags().String("http", "", "HTTP address to listen on (e.g., :8080)")
	serveCmd.Flags().StringArray("allow-path", nil, "Allowed directory for repo_path (can be specified multiple times, defaults to $HOME)")
	rootCmd.AddCommand(serveCmd)
}
