package cmd

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"
	"unicode/utf8"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/salvador-arreola/kubectl-env/pkg/client"
	"github.com/salvador-arreola/kubectl-env/pkg/diff"
)

const truncateAt = 40

var (
	namespace1 string
	namespace2 string
	fullDiff   bool
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Diff ConfigMaps between two contexts",
	RunE:  runDiff,
}

func init() {
	rootCmd.AddCommand(diffCmd)
	diffCmd.Flags().StringVarP(&namespace1, "namespace-1", "n", "default", "namespace for context-1")
	diffCmd.Flags().StringVar(&namespace2, "namespace-2", "", "namespace for context-2 (default: same as --namespace-1)")
	diffCmd.Flags().BoolVar(&fullDiff, "full", false, "show full values via $DIFFTOOL (default: diff)")
}

func runDiff(cmd *cobra.Command, args []string) error {
	if context2 == "" {
		return fmt.Errorf("--context-2 is required")
	}
	if namespace2 == "" {
		namespace2 = namespace1
	}

	ctx1, err := client.ResolveContextName(context1)
	if err != nil {
		return fmt.Errorf("context-1: %w", err)
	}
	ctx2, err := client.ResolveContextName(context2)
	if err != nil {
		return fmt.Errorf("context-2: %w", err)
	}
	if ctx1 == ctx2 && namespace1 == namespace2 {
		return fmt.Errorf("context-1 and context-2 resolve to the same context %q and namespace %q — must differ", ctx1, namespace1)
	}

	c1, err := client.New(ctx1)
	if err != nil {
		return fmt.Errorf("context-1: %w", err)
	}
	c2, err := client.New(ctx2)
	if err != nil {
		return fmt.Errorf("context-2: %w", err)
	}

	if err := client.ValidateNamespace(cmd.Context(), c1, namespace1); err != nil {
		return fmt.Errorf("context-1: %w", err)
	}
	if err := client.ValidateNamespace(cmd.Context(), c2, namespace2); err != nil {
		return fmt.Errorf("context-2: %w", err)
	}

	results, err := diff.ConfigMaps(cmd.Context(), c1, c2, namespace1, namespace2)
	if err != nil {
		return err
	}

	if fullDiff {
		return printFull(results)
	}
	printTable(results)
	return nil
}

func truncate(s string) string {
	if strings.ContainsRune(s, '\n') || utf8.RuneCountInString(s) > truncateAt {
		h := sha256.Sum256([]byte(s))
		return fmt.Sprintf("sha256:%x [%dB]", h[:4], len(s))
	}
	return s
}

func cmLabel(r diff.DiffResult) string {
	if r.Namespace1 == r.Namespace2 {
		return r.Namespace1 + "/" + r.Name
	}
	return r.Namespace1 + "/" + r.Name + " → " + r.Namespace2 + "/" + r.Name
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
				fmt.Fprintln(w, "CONFIGMAP\tKEY\tSTATUS\tCONTEXT-1\tCONTEXT-2")
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
			cm := cmLabel(r)
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				cm, k.Key, status,
				truncate(k.Value1), truncate(k.Value2))
		}
	}

	if !hasDiffs {
		fmt.Println("No differences found.")
	}
}

func printFull(results []diff.DiffResult) error {
	tool := os.Getenv("DIFFTOOL")
	if tool == "" {
		tool = "diff"
	}

	for _, r := range results {
		for _, k := range r.Keys {
			if k.Status != diff.StatusModified {
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

			fmt.Printf("\n=== %s  key=%s ===\n", cmLabel(r), k.Key)
			c := exec.Command(tool, f1, f2)
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			// diff exits 1 when files differ — that's expected, not an error
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
