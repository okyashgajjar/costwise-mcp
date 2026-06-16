package cmd

import (
	"fmt"

	"github.com/okyashgajjar/costaffective-mcp/internal/skill"

	"github.com/spf13/cobra"
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Manage the costaffective-session awareness skill",
	Long: `The costaffective-session skill steers AI coding tools to keep the
session cheap: route large output through stash_context/recall, persist durable
facts with remember, and prefer narrow retrieval over reading whole files.

Every MCP client already receives this guidance automatically via the server's
instructions field (no install needed). This command additionally writes each
client's native rules file — Claude SKILL.md, Codex/opencode/OpenClaw AGENTS.md,
Antigravity/Gemini GEMINI.md, and a project-root AGENTS.md that Cursor, Windsurf,
Copilot, Zed and Aider also read.

Examples:
  costaffective skill install                  # Global + project rules files
  costaffective skill install --scope global   # Only per-user files (~/.codex, ~/.gemini, …)
  costaffective skill install --scope project  # Only ./AGENTS.md + ./.claude/skills
  costaffective skill uninstall                # Remove the guidance everywhere
  costaffective skill print                    # Print the guidance for manual placement
`,
}

var skillInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the costaffective-session guidance into each client's rules file",
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, err := skill.ParseScope(mustString(cmd, "scope"))
		if err != nil {
			return err
		}
		results, err := skill.Install(scope)
		if err != nil {
			return err
		}
		cmd.Println("Installed costaffective-session guidance:")
		for _, r := range results {
			cmd.Printf("  %-7s %-9s %s  [%s]\n", r.Scope, r.Target, r.Path, r.Action)
		}
		cmd.Println("\nEvery MCP client also receives this automatically via the server's instructions field.")
		if scope == skill.ScopeBoth {
			cmd.Println("Note: global + project both ship AGENTS.md-family files; a tool reading both loads the policy twice. Use --scope global or --scope project to avoid.")
		}
		return nil
	},
}

var skillUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the costaffective-session guidance from each client's rules file",
	RunE: func(cmd *cobra.Command, args []string) error {
		scope, err := skill.ParseScope(mustString(cmd, "scope"))
		if err != nil {
			return err
		}
		results, err := skill.Uninstall(scope)
		if err != nil {
			return err
		}
		for _, r := range results {
			cmd.Printf("  %-7s %-9s %s  [%s]\n", r.Scope, r.Target, r.Path, r.Action)
		}
		return nil
	},
}

func mustString(cmd *cobra.Command, name string) string {
	v, _ := cmd.Flags().GetString(name)
	return v
}

var skillPrintCmd = &cobra.Command{
	Use:   "print",
	Short: "Print the session guidance (for manual placement in any tool)",
	RunE: func(cmd *cobra.Command, args []string) error {
		full, _ := cmd.Flags().GetBool("full")
		if full {
			fmt.Fprint(cmd.OutOrStdout(), skill.SkillMarkdown())
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), skill.Policy())
		}
		return nil
	},
}

func init() {
	skillInstallCmd.Flags().String("scope", "both", "Where to install: global|project|both")
	skillUninstallCmd.Flags().String("scope", "both", "Where to uninstall from: global|project|both")
	skillPrintCmd.Flags().Bool("full", false, "Print the full SKILL.md (with frontmatter) instead of just the body")

	skillCmd.AddCommand(skillInstallCmd)
	skillCmd.AddCommand(skillUninstallCmd)
	skillCmd.AddCommand(skillPrintCmd)
	rootCmd.AddCommand(skillCmd)
}
