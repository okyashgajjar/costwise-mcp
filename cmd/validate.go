package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/okyashgajjar/costwise-mcp/internal/policy"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate [target-directory]",
	Short: "Validate repository against .costwise.yaml policy",
	Run: func(cmd *cobra.Command, args []string) {
		repoRoot := "."
		if len(args) > 0 {
			repoRoot = args[0]
		}

		absRoot, err := filepath.Abs(repoRoot)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		p, err := policy.ParsePolicy(absRoot)
		if err != nil {
			fmt.Printf("Policy error: %v\n", err)
			os.Exit(1)
		}

		if p == nil {
			fmt.Println("No .costwise.yaml policy found in repository root.")
			os.Exit(0)
		}

		engine := policy.NewEngine(p)

		var totalViolations int
		var filesScanned int

		_ = filepath.WalkDir(absRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				if d != nil && d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == "vendor") {
					return filepath.SkipDir
				}
				return nil
			}
			// Skip non-regular files (symlinks could point outside the repo —
			// e.g. to secrets or /dev/zero) and cap read size against DoS.
			if !d.Type().IsRegular() {
				return nil
			}
			if info, ierr := d.Info(); ierr == nil && info.Size() > 1<<20 {
				return nil
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(absRoot, path)

			res := engine.EvaluateFile(rel, string(data))
			filesScanned++

			if len(res.Violations) > 0 {
				fmt.Printf("❌ %s (Category: %s, Score: %d/100)\n", rel, res.Category, res.Score)
				for _, v := range res.Violations {
					fmt.Printf("   - [Line %d] %s\n", v.Line, v.Message)
				}
				totalViolations += len(res.Violations)
			}
			return nil
		})

		fmt.Printf("\nScanned %d files. Total Violations: %d\n", filesScanned, totalViolations)
		if totalViolations > 0 {
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
