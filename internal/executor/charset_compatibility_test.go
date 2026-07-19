package executor

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type charsetCompatibilityManifest struct {
	SchemaVersion string                     `json:"schemaVersion"`
	Cases         []charsetCompatibilityCase `json:"cases"`
}

type charsetCompatibilityCase struct {
	ID            string `json:"id"`
	BytesBase64   string `json:"bytesBase64"`
	ContentType   string `json:"contentType"`
	ExpectedHTML  string `json:"expectedHTML"`
	ExpectedError string `json:"expectedError"`
}

func TestCharsetCompatibilityManifest(t *testing.T) {
	path := filepath.Join("..", "..", "fixtures", "html-compat", "charset-manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var manifest charsetCompatibilityManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != "2026-07-15" || len(manifest.Cases) == 0 {
		t.Fatalf("manifest contract = %#v", manifest)
	}
	seen := map[string]bool{}
	for _, fixture := range manifest.Cases {
		fixture := fixture
		t.Run(fixture.ID, func(t *testing.T) {
			if fixture.ID == "" || seen[fixture.ID] {
				t.Fatalf("duplicate or empty fixture ID %q", fixture.ID)
			}
			seen[fixture.ID] = true
			body, err := base64.StdEncoding.DecodeString(fixture.BytesBase64)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := decodeHTML(body, fixture.ContentType)
			if fixture.ExpectedError != "" {
				assertExecutionErrorCode(t, err, fixture.ExpectedError)
				return
			}
			if err != nil || decoded != fixture.ExpectedHTML {
				t.Fatalf("decoded = %q, error = %v", decoded, err)
			}
		})
	}
}
