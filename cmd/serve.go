package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/mark3labs/mcp-go/server"
	"github.com/okyashgajjar/costaffective-mcp/internal/mcpserver"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the CostAffective MCP Server",
	RunE: func(cmd *cobra.Command, args []string) error {
		httpAddr, _ := cmd.Flags().GetString("http")

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
	rootCmd.AddCommand(serveCmd)
}
