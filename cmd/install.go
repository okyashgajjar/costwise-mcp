package cmd

import (
	"github.com/okyashgajjar/costwise-mcp/internal/installer"
	_ "github.com/okyashgajjar/costwise-mcp/internal/installer/targets"

	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install CostWise MCP server for AI coding clients",
	Long: `Configures CostWise as an MCP server for your AI coding clients.

The installer will:
  1. Find (or build) the binary and ensure it's at ~/.local/bin/costwise
  2. Detect which AI coding clients are installed
  3. Prompt you to select targets for configuration
  4. Write the correct MCP config with absolute binary paths for each client

By default, the installer uses the currently running binary.
Use --build to compile from source instead.

Supported clients: claude, cursor, opencode, codex, antigravity

Examples:
  costwise install              # Interactive: detect → prompt → install
  costwise install --all        # Configure all supported clients (non-interactive)
  costwise install --target claude  # Configure only Claude Code (non-interactive)
  costwise install --build      # Build binary from source before installing
  costwise install --dry-run    # Show what would be done without making changes
  costwise install --local      # Configure for current project only
  costwise install --yes        # Non-interactive: auto-detect + accept defaults
  costwise install --repair     # Fix stale configs and reinstall binary
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		build, _ := cmd.Flags().GetBool("build")
		targetID, _ := cmd.Flags().GetString("target")
		local, _ := cmd.Flags().GetBool("local")
		yes, _ := cmd.Flags().GetBool("yes")
		repair, _ := cmd.Flags().GetBool("repair")
		noSkill, _ := cmd.Flags().GetBool("no-skill")

		loc := installer.Location("")
		if local {
			loc = installer.LocationLocal
		} else if all || targetID != "" || yes || repair {
			loc = installer.LocationGlobal
		}

		inst := &installer.Installer{
			Build:     build,
			All:       all,
			DryRun:    dryRun,
			TargetID:  targetID,
			Location:  loc,
			Yes:       yes,
			Repair:    repair,
			SkipSkill: noSkill,
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
	installCmd.Flags().Bool("build", false, "Build the CostWise binary from source before installing")
	installCmd.Flags().String("target", "", "Install only for a specific client (claude, cursor, opencode, codex, antigravity)")
	installCmd.Flags().Bool("local", false, "Install for current project only (instead of global)")
	installCmd.Flags().BoolP("yes", "y", false, "Non-interactive: auto-detect and accept defaults")
	installCmd.Flags().Bool("repair", false, "Repair mode: reinstall binary and fix stale configs")
	installCmd.Flags().Bool("no-skill", false, "Skip installing the costwise-session awareness skill")
	rootCmd.AddCommand(installCmd)
}
