// Package main is the autonation CLI entrypoint: a thin wrapper over the
// analysis pipeline - read Terraform, analyze, render diagrams and report.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	catalogdata "github.com/autonation/autonation/catalog"
	"github.com/autonation/autonation/internal/analyze"
)

// Version is the analyzer version, recorded in the report header.
const Version = "0.2.0"

var outDir string

// rootCmd is the CLI entrypoint.
var rootCmd = &cobra.Command{
	Use:   "autonation",
	Short: "Terraform code analyzer",
	Long: "Auto-nation reads Terraform source, shows the architecture it will " +
		"deploy, and reports best-practice recommendations.",
}

// analyzeCmd analyzes Terraform at a path and writes the diagrams and report.
var analyzeCmd = &cobra.Command{
	Use:   "analyze [path]",
	Short: "Analyze Terraform and report the architecture and recommendations",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAnalyze(args[0], outDir)
	},
}

// Execute runs the CLI.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// main is the program entrypoint.
func main() {
	Execute()
}

func init() {
	rootCmd.AddCommand(analyzeCmd)
	analyzeCmd.Flags().StringVarP(&outDir, "out", "o", "", "write diagrams and report to this directory")
}

// runAnalyze is the whole pipeline. It is also what the WASM build mirrors.
func runAnalyze(path, outDir string) error {
	files, err := readTerraform(path)
	if err != nil {
		return fmt.Errorf("read terraform: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("no .tf or .tf.json files found under %s", path)
	}

	cat, err := catalogdata.Load()
	if err != nil {
		return fmt.Errorf("load catalog: %w", err)
	}

	result := analyze.New(cat).Analyze(files)

	printSummary(os.Stdout, result)

	if outDir != "" {
		if err := writeOutput(outDir, result); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
	}
	return nil
}

// readTerraform collects every .tf and .tf.json file at or under path, keyed by
// the path as the user will see it in the report.
func readTerraform(path string) (map[string]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return map[string]string{filepath.Base(path): string(content)}, nil
	}

	files := map[string]string{}
	err = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// A .terraform directory holds downloaded providers, not user code.
			if d.Name() == ".terraform" || d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".tf") && !strings.HasSuffix(name, ".tf.json") {
			return nil
		}
		content, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(path, p)
		if err != nil {
			rel = p
		}
		files[filepath.ToSlash(rel)] = string(content)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// printSummary writes the headline view to stdout so a run with no --out still
// tells the user something useful.
func printSummary(w *os.File, r analyze.Result) {
	fmt.Fprintf(w, "analyzed %d resource(s), %d data source(s), %d module call(s)\n",
		r.Summary.ResourceCount, r.Summary.DataCount, r.Summary.ModuleCallCount)
	if len(r.Summary.Providers) > 0 {
		fmt.Fprintf(w, "providers: %s\n", strings.Join(r.Summary.Providers, ", "))
	}
	critical := 0
	for _, f := range r.Findings {
		if f.Severity == analyze.Critical {
			critical++
		}
	}
	fmt.Fprintf(w, "%d finding(s): %d critical, %d warning\n",
		len(r.Findings), critical, len(r.Findings)-critical)
	for _, e := range r.Errors {
		if e.Line > 0 {
			fmt.Fprintf(w, "parse error: %s:%d: %s\n", e.File, e.Line, e.Message)
			continue
		}
		fmt.Fprintf(w, "parse error: %s: %s\n", e.File, e.Message)
	}
}

// writeOutput writes the diagrams and the report to dir.
func writeOutput(dir string, r analyze.Result) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	writes := map[string]string{
		"architecture.mmd": r.Diagrams.Mermaid,
		"architecture.d2":  r.Diagrams.D2,
		"report.md":        analyze.Report(r),
	}
	paths := make([]string, 0, len(writes))
	for p := range writes {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		full := filepath.Join(dir, p)
		if err := os.WriteFile(full, []byte(writes[p]), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "wrote %s\n", full)
	}
	return nil
}
