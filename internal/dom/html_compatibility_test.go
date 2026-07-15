package dom

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type htmlCompatibilityManifest struct {
	SchemaVersion       string                  `json:"schemaVersion"`
	ApprovedDivergences []any                   `json:"approvedDivergences"`
	Cases               []htmlCompatibilityCase `json:"cases"`
}

type htmlCompatibilityCase struct {
	ID              string                         `json:"id"`
	Input           string                         `json:"input"`
	DecodedEncoding string                         `json:"decodedEncoding"`
	ParserMode      string                         `json:"parserMode"`
	Observations    []htmlCompatibilityObservation `json:"observations"`
}

type htmlCompatibilityObservation struct {
	Selector   string               `json:"selector"`
	Text       []string             `json:"text"`
	InnerHTML  []string             `json:"innerHTML"`
	Attributes []map[string]*string `json:"attributes"`
}

func TestPinnedHTMLCompatibilityManifest(t *testing.T) {
	root := filepath.Join("..", "..")
	data, err := os.ReadFile(filepath.Join(root, "fixtures", "html-compat", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest htmlCompatibilityManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != "2026-07-15" || len(manifest.ApprovedDivergences) != 0 {
		t.Fatalf("manifest contract = %#v", manifest)
	}
	for _, fixture := range manifest.Cases {
		t.Run(fixture.ID, func(t *testing.T) {
			if fixture.DecodedEncoding != "utf-8" || fixture.ParserMode != "document" {
				t.Fatalf("unsupported fixture mode: %#v", fixture)
			}
			documentData, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(fixture.Input)))
			if err != nil {
				t.Fatal(err)
			}
			document := mustParseHTML(t, string(documentData))
			for _, observation := range fixture.Observations {
				nodes := QueryAll(document, mustSelector(t, observation.Selector))
				if observation.Text != nil {
					actual := make([]string, len(nodes))
					for index, node := range nodes {
						actual[index] = node.TextContent()
					}
					if !reflect.DeepEqual(actual, observation.Text) {
						t.Fatalf("%s text = %#v, want %#v", observation.Selector, actual, observation.Text)
					}
				}
				if observation.InnerHTML != nil {
					actual := make([]string, len(nodes))
					for index, node := range nodes {
						actual[index] = node.InnerHTML()
					}
					if !reflect.DeepEqual(actual, observation.InnerHTML) {
						t.Fatalf("%s innerHTML = %#v, want %#v", observation.Selector, actual, observation.InnerHTML)
					}
				}
				if observation.Attributes != nil {
					actual := make([]map[string]*string, len(nodes))
					for index, node := range nodes {
						actual[index] = map[string]*string{}
						for name := range observation.Attributes[index] {
							if value, ok := node.Attr(name); ok {
								actual[index][name] = &value
							} else {
								actual[index][name] = nil
							}
						}
					}
					if !reflect.DeepEqual(actual, observation.Attributes) {
						t.Fatalf("%s attributes = %#v, want %#v", observation.Selector, actual, observation.Attributes)
					}
				}
			}
		})
	}
}
