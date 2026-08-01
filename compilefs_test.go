package scrapekdl_test

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"testing"
	"testing/fstest"

	scrapekdl "github.com/hsblabs/scrape-kdl"
)

//go:embed testdata/compilefs/*.kdl testdata/compilefs/modules/*.kdl testdata/compilefs/modules/nested/*.kdl
var embeddedSpecs embed.FS

const fsExtractor = `import "modules/common.kdl" as="common"
extractor "filesystem" version="2026-07-15" language-version="2026-07-15" {
  source "html" { fetch mode="http" url="https://example.invalid/" }
  field "title" type="string" required=#true { select "h1"; value "text" }
}`

func TestCompileFSResolvesNestedImportsAndMetadata(t *testing.T) {
	files := fstest.MapFS{
		"spec/main.kdl": {Data: []byte(fsExtractor)},
		"spec/modules/common.kdl": {Data: []byte(`import "nested/types.kdl" as="types"
module "common" version="2026-07-15" language-version="2026-07-15" {}`)},
		"spec/modules/nested/types.kdl": {Data: []byte(`module "types" version="2026-07-15" language-version="2026-07-15" {}`)},
	}
	program, diagnostics, err := scrapekdl.CompileFS(context.Background(), files, "spec/main.kdl")
	if err != nil || diagnostics.HasErrors() || program == nil {
		t.Fatalf("CompileFS() = program %#v, diagnostics %#v, error %v", program, diagnostics, err)
	}
	metadata := program.Metadata()
	paths := make([]string, len(metadata.Files))
	for index, file := range metadata.Files {
		paths[index] = file.Path
	}
	want := []string{"main.kdl", "modules/common.kdl", "modules/nested/types.kdl"}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("source files = %v, want %v", paths, want)
	}
}

func TestCompileFSSupportsEmbedFS(t *testing.T) {
	program, diagnostics, err := scrapekdl.CompileFS(context.Background(), embeddedSpecs, "testdata/compilefs/main.kdl")
	if err != nil || diagnostics.HasErrors() || program == nil || program.Name() != "embedded" {
		t.Fatalf("CompileFS(embed.FS) = program %#v, diagnostics %#v, error %v", program, diagnostics, err)
	}
}

func TestCompileFSRejectsInvalidLogicalPaths(t *testing.T) {
	for _, path := range []string{"", ".", "/main.kdl", "../main.kdl", "spec/../main.kdl"} {
		t.Run(path, func(t *testing.T) {
			program, diagnostics, err := scrapekdl.CompileFS(context.Background(), fstest.MapFS{}, path)
			if program != nil || len(diagnostics) != 0 || !errors.Is(err, fs.ErrInvalid) {
				t.Fatalf("CompileFS(%q) = program %#v, diagnostics %#v, error %v", path, program, diagnostics, err)
			}
		})
	}
}

func TestCompileFSRejectsNilFilesystem(t *testing.T) {
	program, diagnostics, err := scrapekdl.CompileFS(context.Background(), nil, "main.kdl")
	if program != nil || len(diagnostics) != 0 || err == nil {
		t.Fatalf("CompileFS(nil) = program %#v, diagnostics %#v, error %v", program, diagnostics, err)
	}
}

func TestCompileFSRejectsParentEscapingImport(t *testing.T) {
	files := fstest.MapFS{
		"spec/main.kdl": {Data: []byte("import \"../../outside.kdl\" as=\"outside\"\n" + fsExtractor)},
	}
	program, diagnostics, err := scrapekdl.CompileFS(context.Background(), files, "spec/main.kdl")
	if program != nil || len(diagnostics) != 0 || !errors.Is(err, fs.ErrInvalid) {
		t.Fatalf("CompileFS() = program %#v, diagnostics %#v, error %v", program, diagnostics, err)
	}
}

func TestCompileFSRejectsAbsoluteImportAsDocumentDiagnostic(t *testing.T) {
	files := fstest.MapFS{
		"spec/main.kdl": {Data: []byte(`import "/outside.kdl" as="outside"
extractor "absolute-import" version="2026-07-15" language-version="2026-07-15" {
  source "html" { fetch mode="http" url="https://example.invalid/" }
  field "title" type="string" required=#true { select "h1"; value "text" }
}`)},
	}
	program, diagnostics, err := scrapekdl.CompileFS(context.Background(), files, "spec/main.kdl")
	if err != nil || program != nil || !diagnostics.HasErrors() || diagnostics[0].Code != "E_REMOTE_IMPORT_UNSUPPORTED" {
		t.Fatalf("CompileFS() = program %#v, diagnostics %#v, error %v", program, diagnostics, err)
	}
}

func TestCompileFSPreservesCancellationAndFilesystemErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := scrapekdl.CompileFS(ctx, fstest.MapFS{}, "main.kdl")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}

	_, _, err = scrapekdl.CompileFS(context.Background(), fstest.MapFS{}, "missing.kdl")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("filesystem error = %v", err)
	}
}

func TestValidateFSReturnsDocumentDiagnostics(t *testing.T) {
	diagnostics, err := scrapekdl.ValidateFS(context.Background(), fstest.MapFS{
		"invalid.kdl": {Data: []byte("not valid KDL !")},
	}, "invalid.kdl")
	if err != nil || !diagnostics.HasErrors() {
		t.Fatalf("ValidateFS() = diagnostics %#v, error %v", diagnostics, err)
	}
}

func ExampleCompileFS() {
	files := fstest.MapFS{
		"extractor.kdl": {Data: []byte(`extractor "example" version="2026-07-15" language-version="2026-07-15" {
  source "html" { fetch mode="http" url="https://example.invalid/" }
  field "title" type="string" required=#true { select "h1"; value "text" }
}`)},
	}
	program, diagnostics, err := scrapekdl.CompileFS(context.Background(), files, "extractor.kdl")
	fmt.Println(program.Name(), diagnostics.HasErrors(), err)
	// Output: example false <nil>
}
