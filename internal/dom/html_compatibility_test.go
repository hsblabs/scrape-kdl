package dom

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"
)

type htmlCompatibilityManifest struct {
	SchemaVersion       string                  `json:"schemaVersion"`
	Normalization       map[string]any          `json:"normalization"`
	Upstream            htmlCompatibilitySource `json:"upstream"`
	ApprovedDivergences []any                   `json:"approvedDivergences"`
	Cases               []htmlCompatibilityCase `json:"cases"`
}

type htmlCompatibilitySource struct {
	Repository    string   `json:"repository"`
	Revision      string   `json:"revision"`
	SelectedTests []string `json:"selectedTests"`
}
type htmlCompatibilityCase struct {
	ID              string                         `json:"id"`
	Input           string                         `json:"input"`
	DecodedEncoding string                         `json:"decodedEncoding"`
	ParserMode      string                         `json:"parserMode"`
	UpstreamTestID  string                         `json:"upstreamTestId"`
	UpstreamSource  *htmlCompatibilityReduction    `json:"upstreamSource"`
	Observations    []htmlCompatibilityObservation `json:"observations"`
}

type htmlCompatibilityReduction struct {
	Path      string `json:"path"`
	Revision  string `json:"revision"`
	Reduction string `json:"reduction"`
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
	if len(manifest.Normalization) == 0 || manifest.Upstream.Repository != "https://github.com/web-platform-tests/wpt" || !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(manifest.Upstream.Revision) {
		t.Fatalf("manifest provenance = %#v", manifest.Upstream)
	}
	if len(manifest.Cases) < 5 {
		t.Fatalf("manifest cases = %d, want at least 5", len(manifest.Cases))
	}
	if len(manifest.Upstream.SelectedTests) < 3 {
		t.Fatalf("selected upstream tests = %#v", manifest.Upstream.SelectedTests)
	}
	seen := map[string]bool{}
	for _, fixture := range manifest.Cases {
		fixture := fixture
		t.Run(fixture.ID, func(t *testing.T) {
			if fixture.ID == "" || seen[fixture.ID] {
				t.Fatalf("duplicate or empty fixture ID %q", fixture.ID)
			}
			seen[fixture.ID] = true
			if fixture.DecodedEncoding != "utf-8" || fixture.ParserMode != "document" || fixture.UpstreamTestID == "" {
				t.Fatalf("unsupported fixture metadata: %#v", fixture)
			}
			if fixture.UpstreamSource != nil && (fixture.UpstreamSource.Path == "" || fixture.UpstreamSource.Revision != manifest.Upstream.Revision || fixture.UpstreamSource.Reduction == "") {
				t.Fatalf("invalid upstream reduction: %#v", fixture.UpstreamSource)
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
					if len(nodes) != len(observation.Attributes) {
						t.Fatalf("%s nodes = %d, attribute observations = %d", observation.Selector, len(nodes), len(observation.Attributes))
					}
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
