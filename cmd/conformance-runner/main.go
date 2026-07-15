package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/hsblabs/scrape-kdl/internal/conformance"
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("conformance-runner", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { writeHelp(stderr) }
	manifestPath := flags.String("manifest", "conformance/manifest.json", "path to the conformance manifest")
	suite := flags.String("suite", "pr", "focused suite to execute")
	implementation := flags.String("implementation", "go", "implementation to execute (go)")
	job := flags.String("job", "core", "manifest job to execute")
	var output string
	flags.StringVar(&output, "output", "-", "result path, or - for stdout")
	flags.StringVar(&output, "o", "-", "result path, or - for stdout")
	list := flags.Bool("list", false, "list selected cases without executing them")
	if slices.Contains(args, "-h") || slices.Contains(args, "--help") {
		writeHelp(stdout)
		return 0
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "conformance-runner: unexpected positional arguments: %v\n", flags.Args())
		fmt.Fprintln(stderr, "Run conformance-runner --help for usage.")
		return 2
	}
	if *implementation != "go" {
		fmt.Fprintf(stderr, "conformance-runner: implementation %q is not available in the Go runner\n", *implementation)
		fmt.Fprintln(stderr, "Use packages/scrape-kdl/test/manifest-runner.mjs for TypeScript conformance.")
		return 2
	}
	manifest, err := conformance.LoadManifest(*manifestPath)
	if err != nil {
		fmt.Fprintln(stderr, "conformance-runner:", err)
		return 1
	}
	if *list {
		selected, err := manifest.Select(*suite, *implementation, *job)
		if err != nil {
			fmt.Fprintln(stderr, "conformance-runner:", err)
			return 2
		}
		cases := make([]string, len(selected))
		for index, testCase := range selected {
			cases[index] = testCase.ID
		}
		if err := writeJSON(output, stdout, map[string]any{"suite": *suite, "implementation": *implementation, "job": *job, "cases": cases}); err != nil {
			fmt.Fprintln(stderr, "conformance-runner: write case list:", err)
			return 1
		}
		return 0
	}
	result, err := conformance.RunGo(ctx, manifest, *suite, *job)
	if err != nil {
		fmt.Fprintln(stderr, "conformance-runner:", err)
		return 2
	}
	if err := writeJSON(output, stdout, result); err != nil {
		fmt.Fprintln(stderr, "conformance-runner: write result:", err)
		return 1
	}
	if result.Status == "failed" {
		failed := 0
		for _, testCase := range result.Cases {
			if testCase.Status == "failed" {
				failed++
			}
		}
		fmt.Fprintf(stderr, "conformance-runner: %d case(s) have unapproved differences; inspect the JSON result\n", failed)
		return 1
	}
	return 0
}

func writeJSON(path string, stdout io.Writer, value any) error {
	writer := stdout
	var file *os.File
	if path != "-" {
		var err error
		file, err = os.Create(path)
		if err != nil {
			return err
		}
		defer file.Close()
		writer = file
	}
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func writeHelp(writer io.Writer) {
	fmt.Fprintln(writer, "Run registered Scraping KDL conformance cases and emit a stable JSON result.")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Usage:")
	fmt.Fprintln(writer, "  conformance-runner [flags]")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Examples:")
	fmt.Fprintln(writer, "  conformance-runner --suite pr")
	fmt.Fprintln(writer, "  conformance-runner --suite release --job core --output go-result.json")
	fmt.Fprintln(writer, "  conformance-runner --suite browser --job browser-e2e --list")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Flags:")
	fmt.Fprintln(writer, "  -h, --help                 show this help")
	fmt.Fprintln(writer, "      --manifest PATH        manifest path (default: conformance/manifest.json)")
	fmt.Fprintln(writer, "      --suite NAME           focused suite (default: pr)")
	fmt.Fprintln(writer, "      --implementation NAME  implementation to execute (default: go)")
	fmt.Fprintln(writer, "      --job NAME             manifest job (default: core)")
	fmt.Fprintln(writer, "  -o, --output PATH          result path, or - for stdout")
	fmt.Fprintln(writer, "      --list                 list selected cases without executing them")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "JSON results go to stdout unless --output names a file. Errors and guidance go to stderr.")
	fmt.Fprintln(writer, "Repository: https://github.com/hsblabs/scrape-kdl")
}
