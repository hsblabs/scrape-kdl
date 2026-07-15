package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	scrapekdl "github.com/hsblabs/scrape-kdl"
	"github.com/hsblabs/scrape-kdl/internal/dom"
)

const (
	samples    = 7
	iterations = 12
)

func main() {
	source := mustRead("examples/basic-http/extractor.kdl")
	html := mustRead("examples/basic-http/page.html")
	inputsData := mustRead("examples/basic-http/inputs.json")
	irData := mustRead("examples/basic-http/expected-ir.json")
	outputData := mustRead("examples/basic-http/expected-output.json")
	var inputs map[string]any
	must(json.Unmarshal(inputsData, &inputs))

	compile := func() {
		program, diagnostics := scrapekdl.Compile(context.Background(), scrapekdl.Source{Path: "extractor.kdl", Data: source}, scrapekdl.CompileOptions{})
		if diagnostics.HasErrors() || program == nil {
			panic("compile workload failed")
		}
	}
	program, diagnostics := scrapekdl.Compile(context.Background(), scrapekdl.Source{Path: "extractor.kdl", Data: source}, scrapekdl.CompileOptions{})
	if diagnostics.HasErrors() || program == nil {
		panic("prepare workload failed")
	}
	extract := func() {
		result, err := program.ExtractHTML(context.Background(), string(html), scrapekdl.Options{})
		if err != nil || result == nil {
			panic("extract workload failed")
		}
	}
	compileCalibration := func() { var value any; must(json.Unmarshal(irData, &value)) }
	extractCalibration := func() { var value any; must(json.Unmarshal(outputData, &value)) }
	var selectorHTML strings.Builder
	selectorHTML.WriteString("<main>")
	for range 10_000 {
		selectorHTML.WriteString(`<i class="entry"></i>`)
	}
	selectorHTML.WriteString("</main>")
	document, err := dom.ParseHTML(strings.NewReader(selectorHTML.String()))
	must(err)
	selector, err := dom.ParseSelector(".entry")
	must(err)
	selectorAll := func() {
		if len(dom.QueryAll(document, selector)) != 10_000 {
			panic("selector all workload failed")
		}
	}
	selectorFirst := func() {
		if len(dom.QueryLimit(document, selector, 1)) != 1 {
			panic("selector first workload failed")
		}
	}

	result := map[string]float64{
		"compileRatio":        ratio(measure(compile), measure(compileCalibration)),
		"extractRatio":        ratio(measure(extract), measure(extractCalibration)),
		"selectorAllMillis":   milliseconds(measure(selectorAll)),
		"selectorFirstMillis": milliseconds(measure(selectorFirst)),
	}
	encoded, err := json.Marshal(result)
	must(err)
	fmt.Println(string(encoded))
}

func measure(operation func()) time.Duration {
	for range 3 {
		operation()
	}
	values := make([]time.Duration, samples)
	for sample := range samples {
		started := time.Now()
		for range iterations {
			operation()
		}
		values[sample] = time.Since(started) / iterations
	}
	sort.Slice(values, func(left, right int) bool { return values[left] < values[right] })
	return values[len(values)/2]
}

func ratio(workload, calibration time.Duration) float64 {
	return float64(workload) / float64(calibration)
}

func milliseconds(duration time.Duration) float64 {
	return float64(duration) / float64(time.Millisecond)
}

func mustRead(path string) []byte {
	data, err := os.ReadFile(path)
	must(err)
	return data
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
