package cmd

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"
	"unicode/utf8"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/salvador-arreola/kubectl-ctx-diff/pkg/client"
	"github.com/salvador-arreola/kubectl-ctx-diff/pkg/diff"
)

const truncateAt = 40

var (
	namespace1   string
	namespace2   string
	fullDiff     bool
	outputFormat string
	filter       []string
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Diff Kubernetes resources between two contexts",
	RunE:  runDiff,
}

func init() {
	rootCmd.AddCommand(diffCmd)
	diffCmd.Flags().StringVarP(&namespace1, "namespace-1", "n", "default", "namespace for context-1")
	diffCmd.Flags().StringVar(&namespace2, "namespace-2", "", "namespace for context-2 (default: same as --namespace-1)")
	diffCmd.Flags().BoolVar(&fullDiff, "full", false, "show full values via $DIFFTOOL (default: diff)")
	diffCmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "output format: table or json")
	diffCmd.Flags().StringSliceVarP(&filter, "filter", "f", nil, "resource types to include, e.g. configmaps,secrets (default: all)")
}

func runDiff(cmd *cobra.Command, args []string) error {
	if context2 == "" {
		return fmt.Errorf("--context-2 is required")
	}
	if namespace2 == "" {
		namespace2 = namespace1
	}

	ctx1, err := client.ResolveContextName(kubeconfigPath, context1)
	if err != nil {
		return fmt.Errorf("context-1: %w", err)
	}
	ctx2, err := client.ResolveContextName(kubeconfigPath, context2)
	if err != nil {
		return fmt.Errorf("context-2: %w", err)
	}
	if ctx1 == ctx2 && namespace1 == namespace2 {
		return fmt.Errorf("context-1 and context-2 resolve to the same context %q and namespace %q: must differ", ctx1, namespace1)
	}

	// typed clients used only for namespace validation
	c1, err := client.New(kubeconfigPath, ctx1)
	if err != nil {
		return fmt.Errorf("context-1: %w", err)
	}
	c2, err := client.New(kubeconfigPath, ctx2)
	if err != nil {
		return fmt.Errorf("context-2: %w", err)
	}

	ctx := cmd.Context()
	if err := client.ValidateNamespace(ctx, c1, namespace1); err != nil {
		return fmt.Errorf("context-1: %w", err)
	}
	if err := client.ValidateNamespace(ctx, c2, namespace2); err != nil {
		return fmt.Errorf("context-2: %w", err)
	}

	dyn1, err := client.NewDynamic(kubeconfigPath, ctx1)
	if err != nil {
		return fmt.Errorf("context-1: %w", err)
	}
	dyn2, err := client.NewDynamic(kubeconfigPath, ctx2)
	if err != nil {
		return fmt.Errorf("context-2: %w", err)
	}
	disc, err := client.NewDiscovery(kubeconfigPath, ctx1)
	if err != nil {
		return fmt.Errorf("context-1 discovery: %w", err)
	}

	want := buildFilter(filter)
	results, err := diff.AllResources(ctx, dyn1, dyn2, disc, namespace1, namespace2, want)
	if err != nil {
		return err
	}

	switch outputFormat {
	case "json":
		return printJSON(results)
	case "table":
		if fullDiff {
			return printFull(results)
		}
		printTable(results)
		return nil
	default:
		return fmt.Errorf("unknown output format %q: use table or json", outputFormat)
	}
}

// buildFilter returns a function that reports whether a resource type should run.
// Empty filter means run all. Accepts plural resource names or Kind names (case-insensitive).
func buildFilter(f []string) func(string) bool {
	if len(f) == 0 {
		return func(string) bool { return true }
	}
	set := make(map[string]bool, len(f))
	for _, v := range f {
		set[strings.ToLower(v)] = true
	}
	return func(kind string) bool { return set[strings.ToLower(kind)] }
}

func truncate(s string) string {
	if strings.ContainsRune(s, '\n') || utf8.RuneCountInString(s) > truncateAt {
		h := sha256.Sum256([]byte(s))
		return fmt.Sprintf("sha256:%x [%dB]", h[:4], len(s))
	}
	return s
}

func resourceLabel(r diff.DiffResult) string {
	if r.Namespace1 == r.Namespace2 {
		return r.Namespace1 + "/" + r.Name
	}
	return r.Namespace1 + "/" + r.Name + " -> " + r.Namespace2 + "/" + r.Name
}

func printTable(results []diff.DiffResult) {
	added := color.New(color.FgGreen).SprintFunc()
	removed := color.New(color.FgRed).SprintFunc()
	modified := color.New(color.FgYellow).SprintFunc()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	hasDiffs := false
	for _, r := range results {
		for _, k := range r.Keys {
			if k.Status == diff.StatusEqual {
				continue
			}
			if !hasDiffs {
				fmt.Fprintln(w, "KIND\tNAME\tKEY\tSTATUS\tCONTEXT-1\tCONTEXT-2")
				hasDiffs = true
			}
			var status string
			switch k.Status {
			case diff.StatusOnlyIn1:
				status = removed("only-in-1")
			case diff.StatusOnlyIn2:
				status = added("only-in-2")
			case diff.StatusModified:
				status = modified("modified")
			}
			v1, v2 := truncate(k.Value1), truncate(k.Value2)
			if k.Redacted {
				v1, v2 = "[redacted]", "[redacted]"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				r.Kind, resourceLabel(r), k.Key, status, v1, v2)
		}
	}

	if !hasDiffs {
		fmt.Println("No differences found.")
	}
}

func printJSON(results []diff.DiffResult) error {
	filtered := make([]diff.DiffResult, 0, len(results))
	for _, r := range results {
		var keys []diff.KeyDiff
		for _, k := range r.Keys {
			if k.Status != diff.StatusEqual {
				keys = append(keys, k)
			}
		}
		if len(keys) > 0 {
			r.Keys = keys
			filtered = append(filtered, r)
		}
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(filtered)
}

func printFull(results []diff.DiffResult) error {
	tool := os.Getenv("DIFFTOOL")
	if tool == "" {
		tool = "diff"
	}

	for _, r := range results {
		for _, k := range r.Keys {
			if k.Status != diff.StatusModified || k.Redacted {
				continue
			}

			f1, err := writeTmp(fmt.Sprintf("%s_%s_%s_ctx1_*.txt", r.Namespace1, r.Name, k.Key), k.Value1)
			if err != nil {
				return err
			}
			defer os.Remove(f1)

			f2, err := writeTmp(fmt.Sprintf("%s_%s_%s_ctx2_*.txt", r.Namespace2, r.Name, k.Key), k.Value2)
			if err != nil {
				return err
			}
			defer os.Remove(f2)

			fmt.Printf("\n=== %s %s  key=%s ===\n", r.Kind, resourceLabel(r), k.Key)
			c := exec.Command(tool, f1, f2)
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			// diff exits 1 when files differ; that's expected, not an error
			_ = c.Run()
		}
	}
	return nil
}

func writeTmp(pattern, content string) (string, error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return f.Name(), err
}
