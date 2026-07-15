package scripts_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"unicode"
)

type conformanceCoverage struct {
	ContractDate string            `json:"contractDate"`
	Documents    []string          `json:"documents"`
	Rules        []conformanceRule `json:"rules"`
}

type conformanceRule struct {
	ID        string   `json:"id"`
	Source    string   `json:"source"`
	Evidence  []string `json:"evidence"`
	Rationale string   `json:"rationale"`
}

var headingPattern = regexp.MustCompile(`^#{2,3} +(.+?) *#*$`)

func TestConformanceCoverage(t *testing.T) {
	root := filepath.Clean("..")
	manifestPath := filepath.Join(root, "docs/spec/conformance-coverage.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest conformanceCoverage
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode %s: %v", manifestPath, err)
	}
	if manifest.ContractDate != "2026-07-15" {
		t.Fatalf("contract date = %q, want 2026-07-15", manifest.ContractDate)
	}

	wantDocuments := []string{
		"docs/spec/builtins-v0.1.md",
		"docs/spec/diagnostics.md",
		"docs/spec/language-v0.1.md",
		"docs/spec/selectors-v0.1.md",
	}
	gotDocuments := append([]string(nil), manifest.Documents...)
	sort.Strings(gotDocuments)
	if strings.Join(gotDocuments, "\n") != strings.Join(wantDocuments, "\n") {
		t.Fatalf("normative documents = %v, want %v", gotDocuments, wantDocuments)
	}

	headings := make(map[string]struct{})
	for _, document := range manifest.Documents {
		file, err := os.Open(filepath.Join(root, document))
		if err != nil {
			t.Fatal(err)
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			match := headingPattern.FindStringSubmatch(scanner.Text())
			if match == nil {
				continue
			}
			headings[document+"#"+githubAnchor(match[1])] = struct{}{}
		}
		if err := scanner.Err(); err != nil {
			file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}

	seenIDs := make(map[string]struct{})
	seenSources := make(map[string]struct{})
	for _, rule := range manifest.Rules {
		if rule.ID == "" || rule.Source == "" {
			t.Fatalf("coverage entry has empty id or source: %#v", rule)
		}
		if _, exists := seenIDs[rule.ID]; exists {
			t.Errorf("duplicate coverage id %q", rule.ID)
		}
		seenIDs[rule.ID] = struct{}{}
		if _, exists := seenSources[rule.Source]; exists {
			t.Errorf("duplicate coverage source %q", rule.Source)
		}
		seenSources[rule.Source] = struct{}{}
		if _, exists := headings[rule.Source]; !exists {
			t.Errorf("coverage source does not name a normative section: %s", rule.Source)
		}
		hasEvidence := len(rule.Evidence) > 0
		hasRationale := strings.TrimSpace(rule.Rationale) != ""
		if hasEvidence == hasRationale {
			t.Errorf("%s must have evidence or rationale, but not both", rule.ID)
		}
		for _, reference := range rule.Evidence {
			path, _, _ := strings.Cut(reference, "#")
			if _, err := os.Stat(filepath.Join(root, path)); err != nil {
				t.Errorf("%s evidence %q: %v", rule.ID, reference, err)
			}
		}
	}
	for heading := range headings {
		if _, exists := seenSources[heading]; !exists {
			t.Errorf("normative section has no coverage entry: %s", heading)
		}
	}
}

func githubAnchor(heading string) string {
	var output strings.Builder
	previousHyphen := false
	for _, character := range strings.ToLower(strings.ReplaceAll(heading, "`", "")) {
		switch {
		case unicode.IsLetter(character) || unicode.IsDigit(character):
			output.WriteRune(character)
			previousHyphen = false
		case character == ' ' || character == '-':
			if output.Len() > 0 && !previousHyphen {
				output.WriteByte('-')
				previousHyphen = true
			}
		}
	}
	return strings.TrimSuffix(output.String(), "-")
}
