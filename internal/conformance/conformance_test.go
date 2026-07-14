package conformance

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

func TestManifestSchemasInventoryAndGoResult(t *testing.T) {
	manifestPath := filepath.Join("..", "..", "conformance", "manifest.json")
	manifest := loadTestManifest(t, manifestPath)
	manifestSchema := compileTestSchema(t, filepath.Join("..", "..", "conformance", "manifest.schema.json"), "https://hsblabs.github.io/scrape-kdl/conformance/2026-07-15/manifest.schema.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	manifestValue, err := jsonschema.UnmarshalJSON(bytes.NewReader(manifestData))
	if err != nil {
		t.Fatal(err)
	}
	if err := manifestSchema.Validate(manifestValue); err != nil {
		t.Fatalf("manifest schema validation: %v", err)
	}

	result, err := RunGo(context.Background(), manifest, "pr", "core")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "passed" {
		t.Fatalf("Go conformance result = %#v", result)
	}
	resultData, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	resultValue, err := jsonschema.UnmarshalJSON(bytes.NewReader(resultData))
	if err != nil {
		t.Fatal(err)
	}
	resultSchema := compileTestSchema(t, filepath.Join("..", "..", "conformance", "result.schema.json"), "https://hsblabs.github.io/scrape-kdl/conformance/2026-07-15/result.schema.json")
	if err := resultSchema.Validate(resultValue); err != nil {
		t.Fatalf("result schema validation: %v", err)
	}
}

func TestManifestSuiteSelection(t *testing.T) {
	manifest := loadTestManifest(t, filepath.Join("..", "..", "conformance", "manifest.json"))
	tests := []struct {
		suite          string
		implementation string
		job            string
		want           int
	}{
		{suite: "pr", implementation: "go", job: "core", want: 15},
		{suite: "release", implementation: "go", job: "core", want: 16},
		{suite: "release", implementation: "go", job: "browser-e2e", want: 1},
		{suite: "typescript-slice", implementation: "typescript", job: "core", want: 8},
		{suite: "invalid", implementation: "go", job: "core", want: 11},
	}
	for _, test := range tests {
		t.Run(test.suite+"/"+test.implementation+"/"+test.job, func(t *testing.T) {
			selected, err := manifest.Select(test.suite, test.implementation, test.job)
			if err != nil || len(selected) != test.want {
				t.Fatalf("Select() = %d cases, %v; want %d", len(selected), err, test.want)
			}
		})
	}
}

func TestManifestRejectsMissingExpectedArtifact(t *testing.T) {
	manifest := loadTestManifest(t, filepath.Join("..", "..", "conformance", "manifest.json"))
	manifest.Cases[0].Expectations.IR = "fixtures/expected-ir/missing.ir.json"
	manifest.Cases[0].Artifacts[1].Path = "fixtures/expected-ir/missing.ir.json"
	if err := manifest.Validate(); err == nil || !strings.Contains(err.Error(), "missing artifact fixtures/expected-ir/missing.ir.json") {
		t.Fatalf("Validate() = %v", err)
	}
}

func TestManifestRejectsUnregisteredFixture(t *testing.T) {
	manifest := loadTestManifest(t, filepath.Join("..", "..", "conformance", "manifest.json"))
	root := t.TempDir()
	if err := os.CopyFS(filepath.Join(root, "fixtures"), os.DirFS(filepath.Join(manifest.Root, "fixtures"))); err != nil {
		t.Fatal(err)
	}
	manifest.Root = root
	if err := os.WriteFile(filepath.Join(root, "fixtures", "unregistered.kdl"), []byte("unregistered\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := manifest.Validate(); err == nil || !strings.Contains(err.Error(), "unregistered fixture fixtures/unregistered.kdl") {
		t.Fatalf("Validate() = %v", err)
	}
}

func TestManifestRejectsMissingInventoryArtifact(t *testing.T) {
	manifest := loadTestManifest(t, filepath.Join("..", "..", "conformance", "manifest.json"))
	manifest.FixtureInventories[0].Artifacts[0] = "fixtures/parser/missing.json"
	if err := manifest.Validate(); err == nil || !strings.Contains(err.Error(), "missing artifact fixtures/parser/missing.json") {
		t.Fatalf("Validate() = %v", err)
	}
}

func TestManifestRejectsPortableReleaseBlockingDivergence(t *testing.T) {
	manifest := loadTestManifest(t, filepath.Join("..", "..", "conformance", "manifest.json"))
	manifest.ApprovedDivergences = append(manifest.ApprovedDivergences, Divergence{
		ID: "DIV-0001", Case: "valid.basic-http", Observation: "runtime",
		Implementations: []string{"go", "typescript"}, ContractExclusion: "not actually excluded", Rationale: "test", Owner: "test",
	})
	if err := manifest.Validate(); err == nil || !strings.Contains(err.Error(), "portable release-blocking observations cannot be approved") {
		t.Fatalf("Validate() = %v", err)
	}
}

func TestUnapprovedAndApprovedDifferences(t *testing.T) {
	manifest := loadTestManifest(t, filepath.Join("..", "..", "conformance", "manifest.json"))
	manifest.Cases[0].Expectations.Output = "fixtures/inputs/basic-http.json"
	result, err := RunGo(context.Background(), manifest, "runtime", "core")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "failed" || result.Cases[0].Differences[0].ApprovedBy != "" {
		t.Fatalf("unapproved result = %#v", result)
	}
	manifest.ApprovedDivergences = append(manifest.ApprovedDivergences, Divergence{
		ID: "DIV-0001", Case: "valid.basic-http", Observation: "runtime",
		Implementations: []string{"go", "typescript"}, ContractExclusion: "test-only exclusion", Rationale: "exercise approval mechanics", Owner: "test",
	})
	result, err = RunGo(context.Background(), manifest, "runtime", "core")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "passed" || result.Cases[0].Differences[0].ApprovedBy != "DIV-0001" {
		t.Fatalf("approved result = %#v", result)
	}
}

func loadTestManifest(t *testing.T, path string) *Manifest {
	t.Helper()
	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}

func compileTestSchema(t *testing.T, path, url string) *jsonschema.Schema {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	if err := compiler.AddResource(url, document); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile(url)
	if err != nil {
		t.Fatal(err)
	}
	return schema
}
