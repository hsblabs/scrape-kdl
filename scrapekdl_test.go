package scrapekdl_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	scrapekdl "github.com/hsblabs/scrape-kdl"
)

func TestPublicCompileAndExtract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte(`<h1>Hello</h1>`))
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "example.kdl")
	spec := fmt.Sprintf(`extractor "example" version=1 {
  source "html" { fetch mode="http" url=%q }
  field "title" type="string" required=#true {
    select "h1"
    value "text"
  }
}`, server.URL)
	if err := os.WriteFile(path, []byte(spec), 0o600); err != nil {
		t.Fatal(err)
	}
	program, diagnostics := scrapekdl.CompileFile(path)
	if diagnostics.HasErrors() {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	result, err := program.Extract(context.Background(), nil, scrapekdl.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Value["title"] != "Hello" {
		t.Fatalf("result = %#v", result)
	}
}
