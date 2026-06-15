package cmd

import (
	"errors"
	"fmt"

	"github.com/okyashgajjar/costaffective-mcp/internal/updater"

	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update costaffective to the latest released version",
	Long: `Checks GitHub for the latest release and, if it is newer than the
running binary, downloads the build for this platform and replaces the current
executable in place.

Examples:
  costaffective update           # update if a newer version is available
  costaffective update --check   # only report whether an update exists`,
	RunE: func(cmd *cobra.Command, args []string) error {
		checkOnly, _ := cmd.Flags().GetBool("check")

		if checkOnly {
			rel, err := updater.FetchLatest()
			if err != nil {
				return fmt.Errorf("check failed: %w", err)
			}
			if updater.IsNewer(rel.TagName, version) {
				cmd.Printf("Update available: %s (current: %s)\n%s\n", rel.TagName, version, rel.HTMLURL)
			} else {
				cmd.Printf("Up to date (current: %s, latest: %s)\n", version, rel.TagName)
			}
			return nil
		}

		cmd.Printf("Current version: %s — checking for updates...\n", version)
		newVer, err := updater.Update(version)
		if errors.Is(err, updater.ErrUpToDate) {
			cmd.Printf("Already up to date (%s)\n", newVer)
			return nil
		}
		if err != nil {
			return fmt.Errorf("update failed: %w", err)
		}
		cmd.Printf("Updated to %s.\nRestart any running MCP servers (or your editor) to use the new version.\n", newVer)
		return nil
	},
}

func init() {
	updateCmd.Flags().Bool("check", false, "Only check for an update; do not install")
	rootCmd.AddCommand(updateCmd)
}
