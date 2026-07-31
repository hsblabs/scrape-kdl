package apiconsumer

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	scrapekdl "github.com/hsblabs/scrape-kdl"
)

func TestIndependentGoConsumer(t *testing.T) {
	ctx := context.Background()
	files := map[string][]byte{
		"spec/common.kdl": []byte(`module "common" version="2026-07-15" language-version="2026-07-15" {}`),
	}
	program, diagnostics, err := scrapekdl.Compile(ctx, scrapekdl.Source{
		Path: "spec/main.kdl",
		Data: []byte(`import "common.kdl" as="common"
extractor "consumer" version="2026-07-15" language-version="2026-07-15" {
  source "html" { fetch mode="http" url="https://example.invalid/" }
  field "title" type="string" required=#true { select "h1"; value "text" }
}`),
	}, scrapekdl.CompileOptions{Loader: func(_ context.Context, path string) ([]byte, error) {
		data, ok := files[path]
		if !ok {
			return nil, fmt.Errorf("missing source %q", path)
		}
		return data, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	if diagnostics.HasErrors() || program == nil {
		t.Fatalf("compile diagnostics = %#v", diagnostics)
	}

	metadata := program.Metadata()
	if metadata.Name != "consumer" || metadata.LanguageVersion != "2026-07-15" || len(metadata.Files) != 2 {
		t.Fatalf("metadata = %#v", metadata)
	}
	publicTarget, err := url.Parse("https://8.8.8.8/")
	if err != nil {
		t.Fatal(err)
	}
	publicInternetPolicy := scrapekdl.PublicInternetURLPolicy()
	if err := publicInternetPolicy(ctx, publicTarget); err != nil {
		t.Fatalf("PublicInternetURLPolicy(public target) = %v", err)
	}
	httpClient := scrapekdl.NewPublicInternetHTTPClient()
	if httpClient == nil || httpClient.Transport == nil {
		t.Fatal("NewPublicInternetHTTPClient returned an unusable client")
	}
	options := scrapekdl.Options{
		ExternalTransforms: map[string]scrapekdl.ExternalTransform{
			"decorate": func(_ context.Context, input any) (any, error) { return input, nil },
		},
		URLPolicy:  publicInternetPolicy,
		HTTPClient: httpClient,
	}
	result, err := program.ExtractHTML(ctx, "<h1>Example</h1>", options)
	if err != nil || result.Value["title"] != "Example" {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
}
