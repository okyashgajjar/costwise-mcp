package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/okyashgajjar/costwise-mcp/internal/benchmark"

	"github.com/spf13/cobra"
)

var benchCmd = &cobra.Command{
	Use:   "bench",
	Short: "Run ground-truth benchmarks against a repository and print an accuracy/token scorecard",
	Long: `Runs the benchmark datasets under benchmarks/<name> against a checked-out
repository, exercising the same retrieval+compression path as the search_code
tool, and reports per-category file-hit accuracy, routing accuracy, and average
tokens per query.

Examples:
  costwise bench --repo . --dataset benchmarks/costwise
  costwise bench --repo ./targets/aider --dataset benchmarks/aider
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath, _ := cmd.Flags().GetString("repo")
		datasetDir, _ := cmd.Flags().GetString("dataset")
		if repoPath == "" || datasetDir == "" {
			return fmt.Errorf("--repo and --dataset are required")
		}

		absRepo, err := filepath.Abs(repoPath)
		if err != nil {
			return err
		}
		if _, err := os.Stat(absRepo); err != nil {
			return fmt.Errorf("repo path not found: %s", absRepo)
		}

		tasks, err := benchmark.LoadDir(datasetDir)
		if err != nil {
			return fmt.Errorf("load dataset: %w", err)
		}
		if len(tasks) == 0 {
			return fmt.Errorf("no tasks found in %s", datasetDir)
		}

		if clean, _ := cmd.Flags().GetBool("clean"); clean {
			before := len(tasks)
			var dep, stale, minified int
			tasks, dep, stale, minified = benchmark.CleanTasks(tasks, absRepo)
			fmt.Printf("Clean mode: dropped %d/%d junk entries (%d in deps, %d stale paths, %d minified symbols)\n",
				before-len(tasks), before, dep, stale, minified)
		}

		if limit, _ := cmd.Flags().GetInt("limit"); limit > 0 && len(tasks) > limit {
			tasks = benchmark.SampleTasks(tasks, limit)
		}

		fmt.Printf("Indexing %s and running %d prompts from %s...\n", absRepo, len(tasks), datasetDir)

		if compare, _ := cmd.Flags().GetBool("compare"); compare {
			rows, err := benchmark.RunCompare(context.Background(), absRepo, tasks)
			if err != nil {
				return fmt.Errorf("compare: %w", err)
			}
			fmt.Println(benchmark.CompareReport(filepath.Base(datasetDir), rows))
			return nil
		}

		results, err := benchmark.RunDataset(context.Background(), absRepo, tasks)
		if err != nil {
			return fmt.Errorf("run: %w", err)
		}

		fmt.Println(benchmark.Report(filepath.Base(datasetDir), results))
		return nil
	},
}

func init() {
	benchCmd.Flags().String("repo", "", "Path to the checked-out repository to benchmark against")
	benchCmd.Flags().String("dataset", "", "Path to the benchmark dataset dir (e.g. benchmarks/aider)")
	benchCmd.Flags().Bool("clean", false, "Drop low-quality entries (deps/node_modules, stale paths, minified symbols) before scoring")
	benchCmd.Flags().Bool("compare", false, "Compare input-token cost with our MCP vs the default grep+read agent loop")
	benchCmd.Flags().Int("limit", 0, "Cap the number of prompts (0 = all), sampled evenly across categories")
	rootCmd.AddCommand(benchCmd)
}
