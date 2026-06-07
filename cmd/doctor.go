package cmd

import (
	"fmt"
	"os"

	"github.com/okyashgajjar/costaffective-mcp/internal/doctor"

	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose installation and MCP configuration issues",
	Long: `Runs a series of diagnostic checks to verify
that CostAffective is installed and configured correctly.

Checks performed:
  - Binary: exists, executable, version
  - PATH: binary discoverable in shell PATH
  - MCP Configs: config files exist, valid JSON, binary path is valid
  - MCP Startup: server starts and responds to JSON-RPC initialize
  - Repository: directory readable, index directory writable

Exit code: 0 if all checks pass, 1 if any FAIL, 2 if any WARN only.

Examples:
  costaffective doctor
  costaffective doctor --verbose
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		verbose, _ := cmd.Flags().GetBool("verbose")

		fmt.Println("CostAffective Doctor")
		fmt.Println()

		results := doctor.RunAll()

		for _, r := range results {
			if verbose {
				fmt.Println(r.StringVerbose())
			} else {
				fmt.Println(r.String())
			}
		}

		fmt.Println()

		status, passCount, failCount := doctor.FinalStatus(results)
		warnCount := len(results) - passCount - failCount

		fmt.Printf("Results: %d PASS, %d WARN, %d FAIL\n", passCount, warnCount, failCount)

		if failCount > 0 {
			fmt.Println()
			fmt.Println("Issues found. Run:")
			fmt.Println("  costaffective install --repair")
			os.Exit(1)
		} else if warnCount > 0 {
			fmt.Println()
			fmt.Println("All systems functional with warnings.")
			os.Exit(2)
		} else {
			fmt.Printf("\nStatus: %s\n", status)
		}

		return nil
	},
}

func init() {
	doctorCmd.Flags().BoolP("verbose", "v", false, "Show detailed output for each check")
	rootCmd.AddCommand(doctorCmd)
}
