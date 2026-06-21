package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

var versionTemplate = fmt.Sprintf(`{{with .Name}}{{printf "%%s " .}}{{end}}{{printf "%%s" .Version}}
commit: %s
built:  %s
`, commit, date)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "costwise",
	Version: version,
	Short:   "Code Intelligence Research Platform",
	Long: `costwise is a Code Intelligence Research Platform providing
MCP (Model Context Protocol) server for AI coding clients.

It provides:
  - Repository-aware retrieval pipeline
  - MCP server for AI coding assistants
  - Multi-client installation and configuration
  - Comprehensive diagnostics`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	rootCmd.SetVersionTemplate(versionTemplate)
	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
}
