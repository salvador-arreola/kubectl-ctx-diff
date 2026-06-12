package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	context1       string
	context2       string
	kubeconfigPath string
)

var rootCmd = &cobra.Command{
	Short:         "Compare Kubernetes environments across contexts",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	// When invoked as a kubectl plugin (binary starts with "kubectl-"),
	// display "kubectl ctx-diff" in help instead of "kubectl-ctx-diff".
	name := filepath.Base(os.Args[0])
	if strings.HasPrefix(name, "kubectl-") {
		name = "kubectl " + strings.TrimPrefix(name, "kubectl-")
	}
	rootCmd.Use = name

	// Cobra derives CommandPath() from the first word of Use, which breaks
	// when Use contains spaces. Patch the template to use {{.Use}} directly.
	tmpl := rootCmd.UsageTemplate()
	tmpl = strings.ReplaceAll(tmpl, "{{.CommandPath}}", "{{.Use}}")
	rootCmd.SetUsageTemplate(tmpl)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&context1, "context-1", "", "first kubeconfig context (default: current-context)")
	rootCmd.PersistentFlags().StringVar(&context2, "context-2", "", "second kubeconfig context (required)")
	rootCmd.PersistentFlags().StringVar(&kubeconfigPath, "kubeconfig", "", "path to kubeconfig file (default: $KUBECONFIG or ~/.kube/config)")
}
