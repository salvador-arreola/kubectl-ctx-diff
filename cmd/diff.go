package cmd

import (
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
	namespace string
	fullDiff  bool
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Diff ConfigMaps between two contexts",
	RunE:  runDiff,
}

func init() {
	rootCmd.AddCommand(diffCmd)
	diffCmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "namespace to compare")
	diffCmd.Flags().BoolVar(&fullDiff, "full", false, "show full values via $DIFFTOOL (default: diff)")
}

func runDiff(cmd *cobra.Command, args []string) error {
	if context2 == "" {
		return fmt.Errorf("--context-2 is required")
	}

	c1, err := client.New(context1)
	if err != nil {
		return fmt.Errorf("context-1: %w", err)
	}
	c2, err := client.New(context2)
	if err != nil {
		return fmt.Errorf("context-2: %w", err)
	}

	results, err := diff.ConfigMaps(cmd.Context(), c1, c2, namespace)
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
	s = strings.ReplaceAll(s, "\n", "\\n")
	if utf8.RuneCountInString(s) <= truncateAt {
		return s
	}
	return string([]rune(s)[:truncateAt]) + fmt.Sprintf("… [%dB]", len(s))
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
			fmt.Fprintf(w, "%s/%s\t%s\t%s\t%s\t%s\n",
				r.Namespace, r.Name, k.Key, status,
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

			f1, err := writeTmp(fmt.Sprintf("%s_%s_%s_ctx1_*.txt", r.Namespace, r.Name, k.Key), k.Value1)
			if err != nil {
				return err
			}
			defer os.Remove(f1)

			f2, err := writeTmp(fmt.Sprintf("%s_%s_%s_ctx2_*.txt", r.Namespace, r.Name, k.Key), k.Value2)
			if err != nil {
				return err
			}
			defer os.Remove(f2)

			fmt.Printf("\n=== %s/%s  key=%s ===\n", r.Namespace, r.Name, k.Key)
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
