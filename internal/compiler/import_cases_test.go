package compiler

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hsblabs/scrape-kdl/internal/diagnostic"
)

func TestSharedImportCases(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "fixtures", "imports", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name             string            `json:"name"`
		Path             string            `json:"path"`
		Source           string            `json:"source"`
		Files            map[string]string `json:"files"`
		Diagnostics      diagnostic.List   `json:"diagnostics"`
		OperationalError string            `json:"operationalError"`
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, testCase := range cases {
		t.Run(testCase.Name, func(t *testing.T) {
			_, got, compileErr := CompileSource(context.Background(), testCase.Path, []byte(testCase.Source), func(_ context.Context, path string) ([]byte, error) {
				source, ok := testCase.Files[filepath.ToSlash(path)]
				if !ok {
					return nil, errors.New("missing source")
				}
				return []byte(source), nil
			})
			if testCase.OperationalError != "" {
				if compileErr == nil || !strings.Contains(compileErr.Error(), testCase.OperationalError) {
					t.Fatalf("operational error = %v, want match %q", compileErr, testCase.OperationalError)
				}
				return
			}
			if compileErr != nil {
				t.Fatal(compileErr)
			}
			if got == nil {
				got = diagnostic.List{}
			}
			gotJSON, _ := json.MarshalIndent(got, "", "  ")
			wantJSON, _ := json.MarshalIndent(testCase.Diagnostics, "", "  ")
			if string(gotJSON) != string(wantJSON) {
				t.Fatalf("diagnostics mismatch\n got: %s\nwant: %s", gotJSON, wantJSON)
			}
		})
	}
}
