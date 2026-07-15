package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/hsblabs/scrape-kdl/internal/exampleharness"
)

func main() {
	update := flag.Bool("update", false, "rewrite reviewed expected IR and output artifacts")
	root := flag.String("root", ".", "repository root")
	flag.Parse()
	report, err := exampleharness.Check(context.Background(), *root, *update)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, path := range report.Updated {
		fmt.Printf("updated: %s\n", path)
	}
	fmt.Printf("examples: %d checked\n", report.Examples)
}
