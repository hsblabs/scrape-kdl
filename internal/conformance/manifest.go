package conformance

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const SchemaVersion = "2026-07-15"

type Manifest struct {
	Schema              string             `json:"$schema"`
	SchemaVersion       string             `json:"schemaVersion"`
	Contract            Contract           `json:"contract"`
	Normalization       map[string]string  `json:"normalization"`
	FixtureInventories  []FixtureInventory `json:"fixtureInventories"`
	Suites              map[string]Suite   `json:"suites"`
	ApprovedDivergences []Divergence       `json:"approvedDivergences"`
	Cases               []Case             `json:"cases"`
	Root                string             `json:"-"`
}

type Contract struct {
	LanguageVersion string `json:"languageVersion"`
	IRVersion       string `json:"irVersion"`
}

type Suite struct {
	Description string `json:"description"`
}

type FixtureInventory struct {
	ID        string   `json:"id"`
	Purpose   string   `json:"purpose"`
	Consumers []string `json:"consumers"`
	Artifacts []string `json:"artifacts"`
}

type Case struct {
	ID           string       `json:"id"`
	Status       string       `json:"status"`
	Categories   []string     `json:"categories"`
	Suites       []string     `json:"suites"`
	Artifacts    []Artifact   `json:"artifacts"`
	Expectations Expectations `json:"expectations"`
	Executions   []Execution  `json:"executions"`
}

type Artifact struct {
	Path string `json:"path"`
	Role string `json:"role"`
}

type Expectations struct {
	Outcome     string                 `json:"outcome"`
	Diagnostics *DiagnosticExpectation `json:"diagnostics,omitempty"`
	IR          string                 `json:"ir,omitempty"`
	Inputs      string                 `json:"inputs,omitempty"`
	HTML        string                 `json:"html,omitempty"`
	Output      string                 `json:"output,omitempty"`
}

type DiagnosticExpectation struct {
	Artifact string `json:"artifact"`
	Key      string `json:"key"`
}

type Execution struct {
	Implementation string   `json:"implementation"`
	Job            string   `json:"job"`
	Stages         []string `json:"stages"`
}

type Divergence struct {
	ID                string   `json:"id"`
	Case              string   `json:"case"`
	Observation       string   `json:"observation"`
	Implementations   []string `json:"implementations"`
	ContractExclusion string   `json:"contractExclusion"`
	Rationale         string   `json:"rationale"`
	Owner             string   `json:"owner"`
}

func LoadManifest(path string) (*Manifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open conformance manifest: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("decode conformance manifest: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("decode conformance manifest: trailing JSON data")
	}
	manifest.Root = filepath.Dir(filepath.Dir(path))
	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func (manifest *Manifest) Validate() error {
	var problems []string
	if manifest.SchemaVersion != SchemaVersion {
		problems = append(problems, fmt.Sprintf("schemaVersion is %q, want %q", manifest.SchemaVersion, SchemaVersion))
	}
	if manifest.Schema != "./manifest.schema.json" {
		problems = append(problems, "$schema must be ./manifest.schema.json")
	}
	if manifest.Contract.LanguageVersion != SchemaVersion || manifest.Contract.IRVersion != SchemaVersion {
		problems = append(problems, "contract versions must both be 2026-07-15")
	}
	wantNormalization := map[string]string{
		"paths":       "repository-relative-forward-slash",
		"sourceBytes": "exact-no-newline-normalization",
		"json":        "canonical-unicode-code-point-order-finite-binary64",
		"diagnostics": "exact-code-severity-message-span-path-order",
		"spans":       "utf8-byte-offset-one-based-line-column-exclusive-end",
	}
	for key, want := range wantNormalization {
		if manifest.Normalization[key] != want {
			problems = append(problems, fmt.Sprintf("normalization.%s is %q, want %q", key, manifest.Normalization[key], want))
		}
	}

	caseIDs := make(map[string]struct{})
	casesByID := make(map[string]Case)
	registeredArtifacts := make(map[string]struct{})
	inventoryIDs := make(map[string]struct{})
	for _, inventory := range manifest.FixtureInventories {
		if inventory.ID == "" || inventory.Purpose == "" {
			problems = append(problems, "fixture inventory id and purpose are required")
		}
		if _, exists := inventoryIDs[inventory.ID]; exists {
			problems = append(problems, "duplicate fixture inventory id "+inventory.ID)
		}
		inventoryIDs[inventory.ID] = struct{}{}
		if len(inventory.Consumers) == 0 || len(inventory.Artifacts) == 0 {
			problems = append(problems, inventory.ID+": fixture inventory requires consumers and artifacts")
		}
		for _, consumer := range inventory.Consumers {
			if consumer != "go" && consumer != "typescript" {
				problems = append(problems, inventory.ID+": unknown fixture consumer "+consumer)
			}
		}
		for _, artifact := range inventory.Artifacts {
			if !validFixturePath(artifact) {
				problems = append(problems, inventory.ID+": artifact path is not normalized: "+artifact)
				continue
			}
			data, err := os.ReadFile(filepath.Join(manifest.Root, filepath.FromSlash(artifact)))
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s: missing artifact %s: %v", inventory.ID, artifact, err))
			} else if !json.Valid(data) {
				problems = append(problems, inventory.ID+": artifact is not valid JSON: "+artifact)
			}
			if _, exists := registeredArtifacts[artifact]; exists {
				problems = append(problems, inventory.ID+": duplicate registered artifact "+artifact)
			}
			registeredArtifacts[artifact] = struct{}{}
		}
	}
	for _, testCase := range manifest.Cases {
		if _, exists := caseIDs[testCase.ID]; exists {
			problems = append(problems, "duplicate case id "+testCase.ID)
		}
		caseIDs[testCase.ID] = struct{}{}
		casesByID[testCase.ID] = testCase
		if testCase.Status != "release-blocking" && testCase.Status != "provisional" {
			problems = append(problems, testCase.ID+": status must be release-blocking or provisional")
		}
		if testCase.Status == "release-blocking" && !slices.Contains(testCase.Suites, "release") {
			problems = append(problems, testCase.ID+": release-blocking case is absent from release suite")
		}
		for _, suite := range testCase.Suites {
			if _, exists := manifest.Suites[suite]; !exists {
				problems = append(problems, testCase.ID+": unknown suite "+suite)
			}
		}
		artifactPaths := make(map[string]struct{})
		sourceCount := 0
		for _, artifact := range testCase.Artifacts {
			if artifact.Role == "source" {
				sourceCount++
			}
			if !validFixturePath(artifact.Path) {
				problems = append(problems, testCase.ID+": artifact path is not normalized: "+artifact.Path)
				continue
			}
			if _, err := os.Stat(filepath.Join(manifest.Root, filepath.FromSlash(artifact.Path))); err != nil {
				problems = append(problems, fmt.Sprintf("%s: missing artifact %s: %v", testCase.ID, artifact.Path, err))
			} else if strings.Contains(artifact.Role, "expected-") || artifact.Role == "inputs" {
				data, readErr := os.ReadFile(filepath.Join(manifest.Root, filepath.FromSlash(artifact.Path)))
				if readErr != nil || !json.Valid(data) {
					problems = append(problems, testCase.ID+": artifact is not valid JSON: "+artifact.Path)
				}
			}
			if _, exists := artifactPaths[artifact.Path]; exists {
				problems = append(problems, testCase.ID+": duplicate artifact "+artifact.Path)
			}
			artifactPaths[artifact.Path] = struct{}{}
			registeredArtifacts[artifact.Path] = struct{}{}
		}
		if sourceCount != 1 {
			problems = append(problems, fmt.Sprintf("%s: expected one source artifact, found %d", testCase.ID, sourceCount))
		}
		for _, expectedPath := range expectedArtifactPaths(testCase.Expectations) {
			if _, exists := artifactPaths[expectedPath]; !exists {
				problems = append(problems, testCase.ID+": expected artifact is not registered on the case: "+expectedPath)
			}
		}
		if testCase.Expectations.Outcome == "invalid" && testCase.Expectations.Diagnostics == nil {
			problems = append(problems, testCase.ID+": invalid case requires exact expected diagnostics")
		}
		if testCase.Expectations.Outcome != "valid" && testCase.Expectations.Outcome != "invalid" {
			problems = append(problems, testCase.ID+": outcome must be valid or invalid")
		}
		seenExecutions := make(map[string]struct{})
		for _, execution := range testCase.Executions {
			key := execution.Implementation + "/" + execution.Job
			if _, exists := seenExecutions[key]; exists {
				problems = append(problems, testCase.ID+": duplicate execution "+key)
			}
			seenExecutions[key] = struct{}{}
			if len(execution.Stages) == 0 {
				problems = append(problems, testCase.ID+": execution "+key+" has no stages")
			}
			if execution.Implementation != "go" && execution.Implementation != "typescript" {
				problems = append(problems, testCase.ID+": execution "+key+" has unknown implementation")
			}
			if execution.Job != "core" && execution.Job != "browser-e2e" {
				problems = append(problems, testCase.ID+": execution "+key+" has unknown job")
			}
			seenStages := make(map[string]struct{})
			for _, stage := range execution.Stages {
				if _, exists := seenStages[stage]; exists {
					problems = append(problems, testCase.ID+": execution "+key+" repeats stage "+stage)
				}
				seenStages[stage] = struct{}{}
				if !slices.Contains([]string{"validate", "compile", "ir", "runtime", "browser"}, stage) {
					problems = append(problems, testCase.ID+": execution "+key+" has unknown stage "+stage)
				}
			}
		}
		if expectation := testCase.Expectations.Diagnostics; expectation != nil {
			data, err := os.ReadFile(filepath.Join(manifest.Root, filepath.FromSlash(expectation.Artifact)))
			if err == nil {
				var document map[string]json.RawMessage
				if json.Unmarshal(data, &document) == nil {
					if _, exists := document[expectation.Key]; !exists {
						problems = append(problems, testCase.ID+": expected diagnostic key is missing: "+expectation.Key)
					}
				}
			}
		}
	}

	err := filepath.WalkDir(filepath.Join(manifest.Root, "fixtures"), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(manifest.Root, path)
		if err != nil {
			return err
		}
		normalized := filepath.ToSlash(relative)
		if _, exists := registeredArtifacts[normalized]; !exists {
			problems = append(problems, "unregistered fixture "+normalized)
		}
		return nil
	})
	if err != nil {
		problems = append(problems, "discover fixtures: "+err.Error())
	}

	divergenceIDs := make(map[string]struct{})
	for _, divergence := range manifest.ApprovedDivergences {
		if _, exists := divergenceIDs[divergence.ID]; exists {
			problems = append(problems, "duplicate approved divergence "+divergence.ID)
		}
		divergenceIDs[divergence.ID] = struct{}{}
		testCase, exists := casesByID[divergence.Case]
		if !exists {
			problems = append(problems, divergence.ID+": unknown case "+divergence.Case)
		} else if testCase.Status == "release-blocking" && divergence.Observation != "browser" {
			problems = append(problems, divergence.ID+": portable release-blocking observations cannot be approved as divergences")
		}
		if divergence.ContractExclusion == "" || divergence.Rationale == "" || divergence.Owner == "" {
			problems = append(problems, divergence.ID+": contract exclusion, rationale, and owner are required")
		}
	}

	if len(problems) > 0 {
		slices.Sort(problems)
		return fmt.Errorf("invalid conformance manifest:\n- %s", strings.Join(problems, "\n- "))
	}
	return nil
}

func (manifest *Manifest) Select(suite, implementation, job string) ([]Case, error) {
	if _, exists := manifest.Suites[suite]; !exists {
		return nil, fmt.Errorf("unknown suite %q", suite)
	}
	var selected []Case
	for _, testCase := range manifest.Cases {
		if !slices.Contains(testCase.Suites, suite) {
			continue
		}
		for _, execution := range testCase.Executions {
			if execution.Implementation == implementation && execution.Job == job {
				selected = append(selected, testCase)
				break
			}
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("suite %q selects no %s/%s executions", suite, implementation, job)
	}
	return selected, nil
}

func (testCase Case) Execution(implementation, job string) (Execution, bool) {
	for _, execution := range testCase.Executions {
		if execution.Implementation == implementation && execution.Job == job {
			return execution, true
		}
	}
	return Execution{}, false
}

func (testCase Case) SourcePath() string {
	for _, artifact := range testCase.Artifacts {
		if artifact.Role == "source" {
			return artifact.Path
		}
	}
	return ""
}

func validFixturePath(path string) bool {
	return strings.HasPrefix(path, "fixtures/") && filepath.ToSlash(filepath.Clean(path)) == path && !strings.Contains(path, "..")
}

func expectedArtifactPaths(expectations Expectations) []string {
	var paths []string
	if expectations.Diagnostics != nil {
		paths = append(paths, expectations.Diagnostics.Artifact)
	}
	for _, path := range []string{expectations.IR, expectations.Inputs, expectations.HTML, expectations.Output} {
		if path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}
