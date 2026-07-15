package kdl

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hsblabs/scrape-kdl/internal/diagnostic"
)

func TestSharedParserCases(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "fixtures", "parser", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name        string          `json:"name"`
		Source      string          `json:"source"`
		Diagnostics diagnostic.List `json:"diagnostics"`
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, testCase := range cases {
		t.Run(testCase.Name, func(t *testing.T) {
			_, got := Parse(testCase.Name+".kdl", []byte(testCase.Source))
			if got == nil {
				got = diagnostic.List{}
			}
			gotJSON, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			wantJSON, err := json.Marshal(testCase.Diagnostics)
			if err != nil {
				t.Fatal(err)
			}
			if string(gotJSON) != string(wantJSON) {
				t.Fatalf("diagnostics mismatch\n got: %s\nwant: %s", gotJSON, wantJSON)
			}
		})
	}
}

func TestSharedInvalidUTF8Case(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "fixtures", "parser", "invalid-utf8.json"))
	if err != nil {
		t.Fatal(err)
	}
	var testCase struct {
		Path        string          `json:"path"`
		Bytes       []byte          `json:"bytes"`
		Diagnostics diagnostic.List `json:"diagnostics"`
	}
	if err := json.Unmarshal(data, &testCase); err != nil {
		t.Fatal(err)
	}
	_, got := Parse(testCase.Path, testCase.Bytes)
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(testCase.Diagnostics)
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("diagnostics mismatch\n got: %s\nwant: %s", gotJSON, wantJSON)
	}
}
