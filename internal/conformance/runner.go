package conformance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	scrapekdl "github.com/hsblabs/scrape-kdl"
	"github.com/hsblabs/scrape-kdl/internal/canonicaljson"
)

type Result struct {
	SchemaVersion   string       `json:"schemaVersion"`
	ManifestVersion string       `json:"manifestVersion"`
	Implementation  string       `json:"implementation"`
	Suite           string       `json:"suite"`
	Job             string       `json:"job"`
	Status          string       `json:"status"`
	Cases           []CaseResult `json:"cases"`
}

type CaseResult struct {
	ID           string        `json:"id"`
	Status       string        `json:"status"`
	Observations []Observation `json:"observations"`
	Differences  []Difference  `json:"differences"`
}

type Observation struct {
	Kind  string `json:"kind"`
	Value any    `json:"value"`
}

type Difference struct {
	Kind       string `json:"kind"`
	Message    string `json:"message"`
	ApprovedBy string `json:"approvedBy,omitempty"`
}

func RunGo(ctx context.Context, manifest *Manifest, suite, job string) (*Result, error) {
	selected, err := manifest.Select(suite, "go", job)
	if err != nil {
		return nil, err
	}
	result := &Result{
		SchemaVersion: SchemaVersion, ManifestVersion: manifest.SchemaVersion,
		Implementation: "go", Suite: suite, Job: job, Status: "passed", Cases: make([]CaseResult, 0, len(selected)),
	}
	for _, testCase := range selected {
		caseResult := runGoCase(ctx, manifest, testCase, job)
		approveDifferences(manifest, &caseResult, "go")
		caseResult.Status = "passed"
		for _, difference := range caseResult.Differences {
			if difference.ApprovedBy == "" {
				caseResult.Status = "failed"
				result.Status = "failed"
			}
		}
		result.Cases = append(result.Cases, caseResult)
	}
	return result, nil
}

func runGoCase(ctx context.Context, manifest *Manifest, testCase Case, job string) CaseResult {
	caseResult := CaseResult{ID: testCase.ID, Observations: []Observation{}, Differences: []Difference{}}
	execution, _ := testCase.Execution("go", job)
	if slices.Contains(execution.Stages, "browser") {
		caseResult.Differences = append(caseResult.Differences, Difference{Kind: "browser", Message: "browser observations must be emitted by the owning adapter harness"})
		return caseResult
	}

	sourcePath := filepath.Join(manifest.Root, filepath.FromSlash(testCase.SourcePath()))
	program, diagnostics := scrapekdl.CompileFile(ctx, sourcePath)
	if slices.Contains(execution.Stages, "validate") {
		caseResult.Observations = append(caseResult.Observations, Observation{Kind: "diagnostics", Value: diagnostics})
	}
	if testCase.Expectations.Outcome == "valid" {
		if diagnostics.HasErrors() || program == nil {
			caseResult.Differences = append(caseResult.Differences, Difference{Kind: "diagnostics", Message: "expected valid program without error diagnostics"})
			return caseResult
		}
	} else {
		if !diagnostics.HasErrors() || program != nil {
			caseResult.Differences = append(caseResult.Differences, Difference{Kind: "diagnostics", Message: "expected invalid program with error diagnostics and no IR"})
		}
		compareExpectedDiagnostics(manifest, testCase, diagnostics, &caseResult)
		return caseResult
	}

	if slices.Contains(execution.Stages, "ir") {
		actual, err := program.IRJSON()
		if err != nil {
			caseResult.Differences = append(caseResult.Differences, Difference{Kind: "ir", Message: "encode Go IR: " + err.Error()})
		} else {
			appendJSONObservation(&caseResult, "ir", actual)
			compareJSONArtifact(manifest, testCase.Expectations.IR, actual, "ir", &caseResult)
		}
	}
	if slices.Contains(execution.Stages, "runtime") {
		html, err := os.ReadFile(filepath.Join(manifest.Root, filepath.FromSlash(testCase.Expectations.HTML)))
		if err != nil {
			caseResult.Differences = append(caseResult.Differences, Difference{Kind: "runtime", Message: "read HTML artifact: " + err.Error()})
			return caseResult
		}
		if testCase.Expectations.Inputs != "" {
			if _, err := decodeJSONFile(filepath.Join(manifest.Root, filepath.FromSlash(testCase.Expectations.Inputs))); err != nil {
				caseResult.Differences = append(caseResult.Differences, Difference{Kind: "runtime", Message: "decode inputs artifact: " + err.Error()})
				return caseResult
			}
		}
		extracted, err := program.ExtractHTML(ctx, string(html), scrapekdl.Options{})
		if err != nil {
			caseResult.Differences = append(caseResult.Differences, Difference{Kind: "runtime", Message: "execute offline HTML: " + err.Error()})
			return caseResult
		}
		actual, err := json.Marshal(extracted)
		if err != nil {
			caseResult.Differences = append(caseResult.Differences, Difference{Kind: "runtime", Message: "encode extraction result: " + err.Error()})
			return caseResult
		}
		appendJSONObservation(&caseResult, "runtime", actual)
		compareJSONArtifact(manifest, testCase.Expectations.Output, actual, "runtime", &caseResult)
	}
	return caseResult
}

func compareExpectedDiagnostics(manifest *Manifest, testCase Case, actual scrapekdl.Diagnostics, result *CaseResult) {
	expectation := testCase.Expectations.Diagnostics
	if expectation == nil {
		result.Differences = append(result.Differences, Difference{Kind: "diagnostics", Message: "missing expected diagnostic reference"})
		return
	}
	document, err := decodeJSONFile(filepath.Join(manifest.Root, filepath.FromSlash(expectation.Artifact)))
	if err != nil {
		result.Differences = append(result.Differences, Difference{Kind: "diagnostics", Message: "decode expected diagnostics: " + err.Error()})
		return
	}
	entries, ok := document.(map[string]any)
	if !ok {
		result.Differences = append(result.Differences, Difference{Kind: "diagnostics", Message: "expected diagnostics artifact must be a JSON object"})
		return
	}
	expected, exists := entries[expectation.Key]
	if !exists {
		result.Differences = append(result.Differences, Difference{Kind: "diagnostics", Message: "expected diagnostics key is missing: " + expectation.Key})
		return
	}
	actualJSON, err := json.Marshal(actual)
	if err != nil {
		result.Differences = append(result.Differences, Difference{Kind: "diagnostics", Message: "encode diagnostics: " + err.Error()})
		return
	}
	actualValue, err := decodeJSON(actualJSON)
	if err != nil {
		result.Differences = append(result.Differences, Difference{Kind: "diagnostics", Message: "decode diagnostics: " + err.Error()})
		return
	}
	if !equalCanonicalValues(expected, actualValue) {
		result.Differences = append(result.Differences, Difference{Kind: "diagnostics", Message: "diagnostics differ from " + expectation.Artifact + "#" + expectation.Key})
	}
}

func compareJSONArtifact(manifest *Manifest, expectedPath string, actual []byte, kind string, result *CaseResult) {
	if expectedPath == "" {
		result.Differences = append(result.Differences, Difference{Kind: kind, Message: "missing expected artifact reference"})
		return
	}
	expected, err := os.ReadFile(filepath.Join(manifest.Root, filepath.FromSlash(expectedPath)))
	if err != nil {
		result.Differences = append(result.Differences, Difference{Kind: kind, Message: "read expected artifact: " + err.Error()})
		return
	}
	expectedCanonical, expectedErr := canonicaljson.Canonicalize(expected)
	actualCanonical, actualErr := canonicaljson.Canonicalize(actual)
	if expectedErr != nil || actualErr != nil {
		result.Differences = append(result.Differences, Difference{Kind: kind, Message: fmt.Sprintf("canonicalize comparison: expected=%v actual=%v", expectedErr, actualErr)})
		return
	}
	if !bytes.Equal(expectedCanonical, actualCanonical) {
		result.Differences = append(result.Differences, Difference{Kind: kind, Message: "value differs from " + expectedPath})
	}
}

func appendJSONObservation(result *CaseResult, kind string, encoded []byte) {
	value, err := decodeJSON(encoded)
	if err != nil {
		result.Differences = append(result.Differences, Difference{Kind: kind, Message: "decode observation: " + err.Error()})
		return
	}
	result.Observations = append(result.Observations, Observation{Kind: kind, Value: value})
}

func decodeJSONFile(path string) (any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return decodeJSON(data)
}

func decodeJSON(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("trailing JSON data")
	}
	return value, nil
}

func equalCanonicalValues(left, right any) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	leftCanonical, leftErr := canonicaljson.Canonicalize(leftJSON)
	rightCanonical, rightErr := canonicaljson.Canonicalize(rightJSON)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftCanonical, rightCanonical)
}

func approveDifferences(manifest *Manifest, result *CaseResult, implementation string) {
	for index := range result.Differences {
		for _, divergence := range manifest.ApprovedDivergences {
			if divergence.Case == result.ID && divergence.Observation == result.Differences[index].Kind && slices.Contains(divergence.Implementations, implementation) {
				result.Differences[index].ApprovedBy = divergence.ID
				break
			}
		}
	}
}
