package cmd

import (
	"github.com/okyashgajjar/costaffective-mcp/internal/installer"
	_ "github.com/okyashgajjar/costaffective-mcp/internal/installer/targets"

	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install CostAffective MCP server for AI coding clients",
	Long: `Builds, installs, and configures CostAffective as an MCP server
for your AI coding clients.

The installer will:
  1. Build the binary and install it to ~/.local/bin/costaffective
  2. Detect which AI coding clients are installed
  3. Prompt you to select targets for configuration
  4. Write the correct MCP config with absolute binary paths for each client

Supported clients: claude, cursor, opencode, codex, antigravity

Examples:
  costaffective install              # Interactive: build → detect → prompt → install
  costaffective install --all        # Configure all supported clients (non-interactive)
  costaffective install --target claude  # Configure only Claude Code (non-interactive)
  costaffective install --dry-run    # Show what would be done without making changes
  costaffective install --local      # Configure for current project only
  costaffective install --yes        # Non-interactive: auto-detect + accept defaults
  costaffective install --repair     # Fix stale configs and reinstall binary
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		build, _ := cmd.Flags().GetBool("build")
		targetID, _ := cmd.Flags().GetString("target")
		local, _ := cmd.Flags().GetBool("local")
		yes, _ := cmd.Flags().GetBool("yes")
		repair, _ := cmd.Flags().GetBool("repair")

		loc := installer.Location("")
		if local {
			loc = installer.LocationLocal
		} else if all || targetID != "" || yes || repair {
			loc = installer.LocationGlobal
		}

		inst := &installer.Installer{
			Build:    build,
			All:      all,
			DryRun:   dryRun,
			TargetID: targetID,
			Location: loc,
			Yes:      yes,
			Repair:   repair,
		}

		if dryRun {
			cmd.Println("Dry run — no changes will be made")
			cmd.Println()
		}

		return inst.Run()
	},
}

func init() {
	installCmd.Flags().Bool("all", false, "Install for all supported clients (non-interactive)")
	installCmd.Flags().Bool("dry-run", false, "Show what would be done without making changes")
	installCmd.Flags().Bool("build", true, "Build the costaffective binary before installing")
	installCmd.Flags().String("target", "", "Install only for a specific client (claude, cursor, opencode, codex, antigravity)")
	installCmd.Flags().Bool("local", false, "Install for current project only (instead of global)")
	installCmd.Flags().BoolP("yes", "y", false, "Non-interactive: auto-detect and accept defaults")
	installCmd.Flags().Bool("repair", false, "Repair mode: reinstall binary and fix stale configs")
	rootCmd.AddCommand(installCmd)
}
