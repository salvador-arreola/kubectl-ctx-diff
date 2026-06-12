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
	version        = "dev" // overridden at build time via -ldflags "-X ...cmd.version=vX.Y.Z"
)

var rootCmd = &cobra.Command{
	Short:         "Compare Kubernetes environments across contexts",
	SilenceUsage:  true,
	SilenceErrors: true,
	Version:       version,
}

func Execute() {
	// When invoked as a kubectl plugin (binary starts with "kubectl-"),
	// display "kubectl ctx-diff" in help instead of "kubectl-ctx-diff".
	name := filepath.Base(os.Args[0])
	if strings.HasPrefix(name, "kubectl-") {
		name = "kubectl " + strings.ReplaceAll(strings.TrimPrefix(name, "kubectl-"), "_", "-")
	}
	rootCmd.Use = name

	// Cobra derives CommandPath()/Name() from the first word of Use, which
	// breaks when Use contains spaces. Patch both templates to use {{.Use}}.
	tmpl := rootCmd.UsageTemplate()
	tmpl = strings.ReplaceAll(tmpl, "{{.CommandPath}}", "{{.Use}}")
	rootCmd.SetUsageTemplate(tmpl)
	rootCmd.SetVersionTemplate("{{.Use}} version {{.Version}}\n")

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
