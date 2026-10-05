// Package main is the autonation CLI entrypoint: a thin wrapper over the core
// pipeline - parse spec, resolve, emit, run the policy gate, write the tree.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/autonation/autonation/internal/catalog"
	"github.com/autonation/autonation/internal/emit/diagram"
	"github.com/autonation/autonation/internal/emit/hcl"
	"github.com/autonation/autonation/internal/policy"
	"github.com/autonation/autonation/internal/resolver"
	"github.com/autonation/autonation/internal/spec"
	"github.com/autonation/autonation/internal/workspace"
)

// Version is the generator version, recorded in the manifest.
const Version = "0.1.0"

var (
	specPath string
	outDir   string
)

// rootCmd is the CLI entrypoint.
var rootCmd = &cobra.Command{
	Use:   "autonation",
	Short: "UI-driven Azure Terraform generator",
	Long: "Auto-nation turns an architecture description into production-ready " +
		"Azure Terraform in the user's exact house style.",
}

// generateCmd generates the three-tier tree from a spec.
var generateCmd = &cobra.Command{
	Use:   "generate [spec]",
	Short: "Generate the Terraform tree from a spec",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		specPath = args[0]
		return runGenerate(specPath, outDir)
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
	rootCmd.AddCommand(generateCmd)
	generateCmd.Flags().StringVarP(&outDir, "out", "o", ".", "output directory")
}

// runGenerate is the whole pipeline. It is also what the WASM build calls.
func runGenerate(specPath, outDir string) error {
	raw, err := os.ReadFile(specPath)
	if err != nil {
		return fmt.Errorf("read spec: %w", err)
	}

	bp, err := spec.ParseDocument(raw)
	if err != nil {
		return fmt.Errorf("parse spec: %w", err)
	}

	cat, err := catalog.LoadDefault()
	if err != nil {
		return fmt.Errorf("load catalog: %w", err)
	}

	plan, err := resolver.New(cat).Resolve(bp)
	if err != nil {
		return fmt.Errorf("resolve: %w", err)
	}

	files, err := hcl.New(cat).Emit(plan)
	if err != nil {
		return fmt.Errorf("emit hcl: %w", err)
	}

	fileMap := map[string]string{}
	for _, f := range files.Files {
		fileMap[f.Path] = f.Content
	}

	// Diagrams come from the same IR walk.
	diag := diagram.New(cat)
	fileMap["architecture.mmd"] = diag.Mermaid(plan)
	fileMap["architecture.d2"] = diag.D2(plan)

	// The policy gate runs on every generation.
	result := policy.New(cat).Check(plan, fileMap)
	if !result.Passed {
		for _, f := range result.Findings {
			if f.Severity == policy.Critical {
				fmt.Fprintf(os.Stderr, "  %s: %s\n", f.Rule, f.Message)
			}
		}
		return fmt.Errorf("policy gate failed: %d finding(s)", len(result.Findings))
	}

	manifest := &workspace.Manifest{
		APIVersion:       "autonation/v1",
		GeneratorVersion: Version,
		SpecHash:         workspace.HashSpec(raw),
		Files:            workspace.HashFiles(fileMap),
	}

	if err := workspace.New(outDir).Write(fileMap, manifest); err != nil {
		return fmt.Errorf("write workspace: %w", err)
	}

	fmt.Fprintf(os.Stdout, "generated %d files into %s\n", len(fileMap), outDir)
	return nil
}
