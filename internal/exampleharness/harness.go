package exampleharness

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	scrapekdl "github.com/hsblabs/scrape-kdl"
)

type Manifest struct {
	Schema          string      `json:"$schema"`
	Name            string      `json:"name"`
	Description     string      `json:"description"`
	LanguageVersion string      `json:"languageVersion"`
	IRVersion       string      `json:"irVersion"`
	Source          string      `json:"source"`
	Inputs          string      `json:"inputs"`
	Expected        Expected    `json:"expected"`
	Executions      []Execution `json:"executions"`
	Adapters        []string    `json:"adapters"`
}

type Expected struct {
	IR     string `json:"ir"`
	Output string `json:"output"`
}

type Execution struct {
	Implementation string `json:"implementation"`
	Mode           string `json:"mode"`
	Fixture        string `json:"fixture,omitempty"`
	Adapter        string `json:"adapter,omitempty"`
}

type Report struct {
	Examples int
	Updated  []string
}

func Check(ctx context.Context, root string, update bool) (Report, error) {
	examplesRoot := filepath.Join(root, "examples")
	entries, err := os.ReadDir(examplesRoot)
	if err != nil {
		return Report{}, fmt.Errorf("read examples: %w", err)
	}
	var report Report
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		directory := filepath.Join(examplesRoot, entry.Name())
		updated, err := checkExample(ctx, directory, entry.Name(), update)
		if err != nil {
			return report, fmt.Errorf("example %s: %w", entry.Name(), err)
		}
		report.Examples++
		for _, path := range updated {
			relative, relativeErr := filepath.Rel(root, path)
			if relativeErr != nil {
				return report, relativeErr
			}
			report.Updated = append(report.Updated, filepath.ToSlash(relative))
		}
	}
	if report.Examples == 0 {
		return report, errors.New("no examples found")
	}
	return report, nil
}

func checkExample(ctx context.Context, directory, directoryName string, update bool) ([]string, error) {
	manifest, err := readManifest(filepath.Join(directory, "example.json"))
	if err != nil {
		return nil, err
	}
	if err := validateManifest(directory, directoryName, manifest); err != nil {
		return nil, err
	}
	program, diagnostics, err := scrapekdl.CompileFile(ctx, filepath.Join(directory, manifest.Source))
	if err != nil {
		return nil, fmt.Errorf("compile source: %w", err)
	}
	if diagnostics.HasErrors() || program == nil {
		data, _ := json.Marshal(diagnostics)
		return nil, fmt.Errorf("compile failed: %s", data)
	}
	if program.Name() != manifest.Name {
		return nil, fmt.Errorf("manifest name %q does not match extractor name %q", manifest.Name, program.Name())
	}
	irJSON, err := program.IRJSON()
	if err != nil {
		return nil, fmt.Errorf("marshal IR: %w", err)
	}
	irJSON = append(irJSON, '\n')
	if err := validateCompiledVersions(irJSON, manifest); err != nil {
		return nil, err
	}
	inputs, err := readInputs(filepath.Join(directory, manifest.Inputs))
	if err != nil {
		return nil, err
	}
	var outputJSON []byte
	for index, execution := range manifest.Executions {
		if execution.Implementation != "go" {
			continue
		}
		result, runErr := execute(ctx, directory, program, inputs, execution)
		if runErr != nil {
			return nil, fmt.Errorf("executions[%d]: %w", index, runErr)
		}
		data, marshalErr := json.MarshalIndent(result, "", "  ")
		if marshalErr != nil {
			return nil, fmt.Errorf("executions[%d]: marshal output: %w", index, marshalErr)
		}
		data = append(data, '\n')
		if outputJSON == nil {
			outputJSON = data
		} else if diff := compareJSON(outputJSON, data); diff != "" {
			return nil, fmt.Errorf("executions[%d] differs from the first Go execution: %s", index, diff)
		}
	}
	if outputJSON == nil {
		return nil, errors.New("at least one Go execution is required")
	}

	artifacts := []struct {
		path string
		data []byte
	}{
		{path: filepath.Join(directory, manifest.Expected.IR), data: irJSON},
		{path: filepath.Join(directory, manifest.Expected.Output), data: outputJSON},
	}
	var updated []string
	for _, artifact := range artifacts {
		if update {
			if err := writeGolden(artifact.path, artifact.data); err != nil {
				return nil, err
			}
			updated = append(updated, artifact.path)
			continue
		}
		expected, err := os.ReadFile(artifact.path)
		if err != nil {
			return nil, fmt.Errorf("read golden %s: %w", filepath.Base(artifact.path), err)
		}
		if diff := compareJSON(expected, artifact.data); diff != "" {
			return nil, fmt.Errorf("%s differs: %s (review, then run `go run ./cmd/check-examples --update`)", filepath.Base(artifact.path), diff)
		}
	}
	return updated, nil
}

func readManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	if err := requireEOF(decoder); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	return manifest, nil
}

func validateManifest(directory, directoryName string, manifest Manifest) error {
	if manifest.Schema != "../schema.json" {
		return fmt.Errorf("$schema must be %q", "../schema.json")
	}
	if manifest.Name != directoryName {
		return fmt.Errorf("name %q must match directory %q", manifest.Name, directoryName)
	}
	if strings.TrimSpace(manifest.Description) == "" {
		return errors.New("description is required")
	}
	if !contains(scrapekdl.SupportedLanguageVersions(), manifest.LanguageVersion) {
		return fmt.Errorf("unsupported languageVersion %q", manifest.LanguageVersion)
	}
	if !contains(scrapekdl.SupportedIRVersions(), manifest.IRVersion) {
		return fmt.Errorf("unsupported irVersion %q", manifest.IRVersion)
	}
	paths := []struct {
		name string
		path string
	}{
		{name: "source", path: manifest.Source},
		{name: "inputs", path: manifest.Inputs},
		{name: "expected.ir", path: manifest.Expected.IR},
		{name: "expected.output", path: manifest.Expected.Output},
	}
	for _, item := range paths {
		if err := validateLocalPath(directory, item.name, item.path); err != nil {
			return err
		}
	}
	if len(manifest.Executions) == 0 {
		return errors.New("executions must not be empty")
	}
	for index, execution := range manifest.Executions {
		if execution.Implementation != "go" && execution.Implementation != "typescript" {
			return fmt.Errorf("executions[%d].implementation must be go or typescript", index)
		}
		if execution.Mode != "offline-html" && execution.Mode != "local-http" && execution.Mode != "browser" {
			return fmt.Errorf("executions[%d].mode is invalid", index)
		}
		if execution.Fixture == "" {
			return fmt.Errorf("executions[%d].fixture is required", index)
		}
		if err := validateLocalPath(directory, fmt.Sprintf("executions[%d].fixture", index), execution.Fixture); err != nil {
			return err
		}
		if execution.Mode == "browser" {
			if execution.Adapter == "" || !contains(manifest.Adapters, execution.Adapter) {
				return fmt.Errorf("executions[%d].adapter must name a declared adapter", index)
			}
		} else if execution.Adapter != "" {
			return fmt.Errorf("executions[%d].adapter is only valid in browser mode", index)
		}
	}
	adapters := append([]string(nil), manifest.Adapters...)
	sort.Strings(adapters)
	for index, adapter := range adapters {
		if adapter == "" || index > 0 && adapter == adapters[index-1] {
			return errors.New("adapters must contain unique non-empty names")
		}
	}
	return nil
}

func validateLocalPath(directory, name, value string) error {
	if value == "" || filepath.IsAbs(value) || filepath.Clean(value) != value || value == ".." || strings.HasPrefix(value, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%s must be a clean relative path within the example", name)
	}
	path := filepath.Join(directory, value)
	relative, err := filepath.Rel(directory, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%s escapes the example directory", name)
	}
	return nil
}

func validateCompiledVersions(data []byte, manifest Manifest) error {
	var metadata struct {
		LanguageVersion string `json:"languageVersion"`
		IRVersion       string `json:"irVersion"`
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return fmt.Errorf("decode compiled version metadata: %w", err)
	}
	if metadata.LanguageVersion != manifest.LanguageVersion || metadata.IRVersion != manifest.IRVersion {
		return fmt.Errorf("compiled versions language=%q ir=%q do not match manifest language=%q ir=%q", metadata.LanguageVersion, metadata.IRVersion, manifest.LanguageVersion, manifest.IRVersion)
	}
	return nil
}

func readInputs(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read inputs: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var inputs map[string]any
	if err := decoder.Decode(&inputs); err != nil {
		return nil, fmt.Errorf("decode inputs: %w", err)
	}
	if inputs == nil {
		return nil, errors.New("inputs must be a JSON object")
	}
	if err := requireEOF(decoder); err != nil {
		return nil, fmt.Errorf("decode inputs: %w", err)
	}
	return normalizeNumbers(inputs).(map[string]any), nil
}

func execute(ctx context.Context, directory string, program *scrapekdl.Program, inputs map[string]any, execution Execution) (*scrapekdl.Result, error) {
	html, err := os.ReadFile(filepath.Join(directory, execution.Fixture))
	if err != nil {
		return nil, fmt.Errorf("read fixture: %w", err)
	}
	switch execution.Mode {
	case "offline-html":
		return program.ExtractHTML(ctx, string(html), scrapekdl.Options{})
	case "local-http":
		server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			response.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = response.Write(html)
		}))
		defer server.Close()
		target, err := url.Parse(server.URL)
		if err != nil {
			return nil, err
		}
		transport := &localTransport{target: target, base: http.DefaultTransport.(*http.Transport).Clone()}
		client := &http.Client{Transport: transport}
		defer transport.CloseIdleConnections()
		return program.Extract(ctx, inputs, scrapekdl.Options{HTTPClient: client})
	case "browser":
		return nil, errors.New("Go browser examples require an adapter runner and are not enabled in this harness stage")
	default:
		return nil, fmt.Errorf("unsupported execution mode %q", execution.Mode)
	}
}

type localTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (transport *localTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	urlCopy := *request.URL
	urlCopy.Scheme = transport.target.Scheme
	urlCopy.Host = transport.target.Host
	clone.URL = &urlCopy
	return transport.base.RoundTrip(clone)
}

func (transport *localTransport) CloseIdleConnections() {
	if closer, ok := transport.base.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}

func compareJSON(expectedData, actualData []byte) string {
	expected, err := decodeJSON(expectedData)
	if err != nil {
		return "expected golden is invalid JSON: " + err.Error()
	}
	actual, err := decodeJSON(actualData)
	if err != nil {
		return "actual value is invalid JSON: " + err.Error()
	}
	return firstDifference("$", expected, actual)
}

func decodeJSON(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if err := requireEOF(decoder); err != nil {
		return nil, err
	}
	return value, nil
}

func firstDifference(path string, expected, actual any) string {
	if reflect.TypeOf(expected) != reflect.TypeOf(actual) {
		return fmt.Sprintf("%s: expected %s, got %s", path, render(expected), render(actual))
	}
	switch expectedValue := expected.(type) {
	case map[string]any:
		actualValue := actual.(map[string]any)
		keys := make([]string, 0, len(expectedValue))
		seen := map[string]bool{}
		for key := range expectedValue {
			keys = append(keys, key)
			seen[key] = true
		}
		for key := range actualValue {
			if !seen[key] {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
		for _, key := range keys {
			expectedItem, expectedOK := expectedValue[key]
			actualItem, actualOK := actualValue[key]
			child := path + "." + key
			if !expectedOK {
				return fmt.Sprintf("%s: unexpected value %s", child, render(actualItem))
			}
			if !actualOK {
				return fmt.Sprintf("%s: missing; expected %s", child, render(expectedItem))
			}
			if difference := firstDifference(child, expectedItem, actualItem); difference != "" {
				return difference
			}
		}
		return ""
	case []any:
		actualValue := actual.([]any)
		if len(expectedValue) != len(actualValue) {
			return fmt.Sprintf("%s: expected array length %d, got %d", path, len(expectedValue), len(actualValue))
		}
		for index := range expectedValue {
			if difference := firstDifference(fmt.Sprintf("%s[%d]", path, index), expectedValue[index], actualValue[index]); difference != "" {
				return difference
			}
		}
		return ""
	default:
		if !reflect.DeepEqual(expected, actual) {
			return fmt.Sprintf("%s: expected %s, got %s", path, render(expected), render(actual))
		}
		return ""
	}
}

func render(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(data)
}

func writeGolden(path string, data []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".golden-*")
	if err != nil {
		return fmt.Errorf("create golden: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write golden: %w", err)
	}
	if err := temporary.Chmod(0o644); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("chmod golden: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close golden: %w", err)
	}
	if err := os.Rename(temporaryName, path); err != nil {
		return fmt.Errorf("replace golden: %w", err)
	}
	return nil
}

func requireEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func normalizeNumbers(value any) any {
	switch typed := value.(type) {
	case json.Number:
		if integer, err := typed.Int64(); err == nil {
			return integer
		}
		if float, err := typed.Float64(); err == nil {
			return float
		}
		return typed.String()
	case []any:
		for index := range typed {
			typed[index] = normalizeNumbers(typed[index])
		}
	case map[string]any:
		for key := range typed {
			typed[key] = normalizeNumbers(typed[key])
		}
	}
	return value
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
