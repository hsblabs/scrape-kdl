package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	scrapekdl "github.com/hsblabs/scrape-kdl"
	rodadapter "github.com/hsblabs/scrape-kdl/adapters/rod"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	spec := flag.String("spec", "", "path to an extractor KDL file")
	showVersion := flag.Bool("version", false, "print version and exit")
	allowJS := flag.Bool("allow-js", false, "allow JavaScript from the trusted spec")
	headless := flag.Bool("headless", true, "run Chromium headlessly")
	flag.Parse()
	if *showVersion {
		fmt.Printf("scrape-kdl-rod %s (commit=%s, built=%s)\n", version, commit, date)
		return
	}
	if *spec == "" {
		fmt.Fprintln(os.Stderr, "-spec is required")
		os.Exit(2)
	}

	program, diagnostics := scrapekdl.CompileFile(*spec)
	if diagnostics.HasErrors() {
		_ = json.NewEncoder(os.Stderr).Encode(diagnostics)
		os.Exit(1)
	}

	controlURL := launcher.New().Headless(*headless).MustLaunch()
	browser := rod.New().ControlURL(controlURL).MustConnect()
	defer browser.MustClose()
	adapter, err := rodadapter.NewBrowser(browser)
	if err != nil {
		fatal(err)
	}
	defer adapter.Close()

	result, err := program.Extract(context.Background(), map[string]any{}, scrapekdl.Options{
		Browser: adapter, AllowJavaScript: *allowJS,
	})
	if err != nil {
		fatal(err)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
