package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	context1 string
	context2 string
)

var rootCmd = &cobra.Command{
	Use:          "kubectl-env",
	Short:        "Compare Kubernetes environments across contexts",
	SilenceUsage: true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&context1, "context-1", "", "first kubeconfig context (default: current-context)")
	rootCmd.PersistentFlags().StringVar(&context2, "context-2", "", "second kubeconfig context (required)")
}
