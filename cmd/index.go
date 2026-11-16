package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/okyashgajjar/costwise-mcp/internal/session"
	"github.com/spf13/cobra"
)

var indexHeadless bool

var indexCmd = &cobra.Command{
	Use:   "index [repository-root]",
	Short: "Build the repository index headlessly (CI/CD precompute)",
	Long: `Build the repository retrieval index (.mycli-fts/) without starting
a session or server. Intended for CI/CD cache precompute pipelines.

Use --headless to suppress interactive output and rely on exit codes.`,
	Run: func(cmd *cobra.Command, args []string) {
		repoRoot := "."
		if len(args) > 0 {
			repoRoot = args[0]
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Honor SIGINT/SIGTERM so CI cancellations shut down cleanly.
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		defer signal.Stop(sigCh)
		go func() {
			select {
			case <-sigCh:
				cancel()
			case <-ctx.Done():
			}
		}()

		start := time.Now()
		rs, err := session.NewRepoSession(ctx, repoRoot, "headless-index")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer rs.Close()

		if indexHeadless {
			fmt.Printf("indexed %s in %s\n", repoRoot, time.Since(start).Round(time.Millisecond))
			return
		}

		fmt.Printf("✅ Repository indexed: %s (%s)\n", repoRoot, time.Since(start).Round(time.Millisecond))
		fmt.Println("   Index artifacts written to .mycli-fts/")
	},
}

func init() {
	indexCmd.Flags().BoolVar(&indexHeadless, "headless", false, "minimal output for CI (errors on stderr, non-zero exit on failure)")
	rootCmd.AddCommand(indexCmd)
}
