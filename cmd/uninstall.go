package cmd

import (
	"github.com/okyashgajjar/costaffective-mcp/internal/installer"
	_ "github.com/okyashgajjar/costaffective-mcp/internal/installer/targets"

	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove CostAffective MCP configs from AI coding clients",
	Long: `Removes the costaffective MCP server entry from configured
AI coding clients. Sweeps both unified and legacy config paths.

Examples:
  costaffective uninstall            # Interactive: detect → confirm → remove
  costaffective uninstall --all      # Remove from all supported clients
  costaffective uninstall --target claude  # Remove only from Claude Code
  costaffective uninstall --dry-run  # Show what would be removed without making changes
  costaffective uninstall --yes      # Non-interactive: remove all detected configs
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		targetID, _ := cmd.Flags().GetString("target")
		local, _ := cmd.Flags().GetBool("local")
		yes, _ := cmd.Flags().GetBool("yes")

		loc := installer.LocationGlobal
		if local {
			loc = installer.LocationLocal
		}

		inst := &installer.Installer{
			All:       all,
			DryRun:    dryRun,
			TargetID:  targetID,
			Location:  loc,
			Yes:       yes,
			Uninstall: true,
		}

		if dryRun {
			cmd.Println("Dry run — no changes will be made")
			cmd.Println()
		}

		return inst.Run()
	},
}

func init() {
	uninstallCmd.Flags().Bool("all", false, "Uninstall from all supported clients")
	uninstallCmd.Flags().Bool("dry-run", false, "Show what would be done without making changes")
	uninstallCmd.Flags().String("target", "", "Uninstall only from a specific client (claude, cursor, opencode, codex, antigravity)")
	uninstallCmd.Flags().Bool("local", false, "Uninstall from current project only (instead of global)")
	uninstallCmd.Flags().BoolP("yes", "y", false, "Non-interactive: remove all detected configs without prompting")
	rootCmd.AddCommand(uninstallCmd)
}
