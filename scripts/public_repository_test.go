package scripts

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicRepositoryGuidanceIsLinked(t *testing.T) {
	root := repositoryRoot(t)
	requiredLinks := []struct {
		path string
		link string
	}{
		{path: "README.md", link: "docs/responsible-use.md"},
		{path: "CONTRIBUTING.md", link: "docs/responsible-use.md"},
		{path: "packages/scrape-kdl/README.md", link: "docs/responsible-use.md"},
		{path: "packages/scrape-kdl-playwright/README.md", link: "docs/responsible-use.md"},
	}

	for _, required := range requiredLinks {
		content := readFile(t, filepath.Join(root, required.path))
		if !strings.Contains(content, required.link) {
			t.Errorf("%s does not link to %s", required.path, required.link)
		}
	}

	guidance := readFile(t, filepath.Join(root, "docs", "responsible-use.md"))
	for _, required := range []string{
		"robots.txt",
		"User-Agent",
		"rate limit",
		"copyright",
		"personal information",
		"anti-bot",
	} {
		if !strings.Contains(strings.ToLower(guidance), strings.ToLower(required)) {
			t.Errorf("docs/responsible-use.md does not cover %q", required)
		}
	}
}

func TestPublicRepositoryExamplesDoNotTargetNamedServices(t *testing.T) {
	root := repositoryRoot(t)
	paths := []string{
		"README.md",
		"CONTRIBUTING.md",
		"docs",
		"examples",
		"fixtures",
		"packages",
	}
	for _, path := range paths {
		path := filepath.Join(root, path)
		err := filepath.WalkDir(path, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			lower := strings.ToLower(string(content))
			for _, service := range []string{"netkeiba", "suumo", "kabutan"} {
				if strings.Contains(lower, service) {
					relative, err := filepath.Rel(root, path)
					if err != nil {
						return err
					}
					t.Errorf("%s contains real-service reference %q", relative, service)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
