package scripts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicReleaseWorkflowKeepsPublicChannelSeparate(t *testing.T) {
	root := repositoryRoot(t)
	tests := []struct {
		path     string
		required []string
		forbid   []string
	}{
		{
			path: ".github/workflows/release.yml",
			required: []string{
				"workflow_dispatch:",
				"PUBLISH_RELEASE",
				`REPOSITORY_VISIBILITY" != public`,
				"environment: release-publish",
				"contents: write",
				"NPM_ACCESS=public",
				"id-token: write",
				"./scripts/publish-github-release.sh core",
				"./scripts/publish-npm-release.sh",
				"./scripts/publish-github-release.sh rod",
				"./scripts/wait-public-go-module.sh",
			},
			forbid: []string{
				"--access restricted",
				"NPM_TOKEN",
				"tags: [\"v*.*.*\"]",
				"tags: [\"adapters/rod/v*.*.*\"]",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, test.path))
			if err != nil {
				t.Fatal(err)
			}
			content := string(data)
			for _, required := range test.required {
				if !strings.Contains(content, required) {
					t.Errorf("%s is missing %q", test.path, required)
				}
			}
			for _, forbidden := range test.forbid {
				if strings.Contains(content, forbidden) {
					t.Errorf("%s contains forbidden %q", test.path, forbidden)
				}
			}
		})
	}
}

func TestRodReleaseWorkflowsAttachBinaryArchives(t *testing.T) {
	root := repositoryRoot(t)
	tests := []struct {
		path     string
		required []string
	}{
		{
			path: ".github/workflows/release.yml",
			required: []string{
				"./scripts/build-rod-release.sh",
				"./scripts/publish-github-release.sh rod",
			},
		},
	}
	for _, test := range tests {
		data, err := os.ReadFile(filepath.Join(root, test.path))
		if err != nil {
			t.Fatal(err)
		}
		content := string(data)
		for _, required := range test.required {
			if !strings.Contains(content, required) {
				t.Errorf("%s is missing %q", test.path, required)
			}
		}
	}
}

func TestPublicReleaseHasNoObsoletePublicationWorkflows(t *testing.T) {
	root := repositoryRoot(t)
	for _, path := range []string{
		".github/workflows/release-npm.yml",
		".github/workflows/release-rod.yml",
		".github/workflows/release-npm-private.yml",
		".github/workflows/release-private.yml",
		".github/workflows/release-private-rod.yml",
	} {
		if _, err := os.Stat(filepath.Join(root, path)); !os.IsNotExist(err) {
			t.Errorf("obsolete public workflow still exists: %s", path)
		}
	}
	content := readFile(t, filepath.Join(root, ".github/workflows/release.yml"))
	if count := strings.Count(content, "environment: release-publish"); count != 1 {
		t.Errorf("release-publish Environment count = %d; want 1", count)
	}
	for _, forbidden := range []string{"npx ", "on:\n  push:", "NPM_TOKEN"} {
		if strings.Contains(content, forbidden) {
			t.Errorf("unified public workflow contains forbidden %q", forbidden)
		}
	}
}

func TestPublicReleasePublishesGoTagsBeforeProxyLookup(t *testing.T) {
	root := repositoryRoot(t)
	content := readFile(t, filepath.Join(root, ".github/workflows/release.yml"))
	tests := []struct {
		name    string
		publish string
		lookup  string
	}{
		{
			name:    "core",
			publish: `./scripts/publish-github-release.sh core "$RELEASE_TAG"`,
			lookup:  `./scripts/wait-public-go-module.sh github.com/hsblabs/scrape-kdl "$RELEASE_TAG"`,
		},
		{
			name:    "go-rod",
			publish: `./scripts/publish-github-release.sh rod "$ROD_TAG"`,
			lookup:  `./scripts/wait-public-go-module.sh github.com/hsblabs/scrape-kdl/adapters/rod "$RELEASE_TAG"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			publishIndex := strings.Index(content, test.publish)
			lookupIndex := strings.Index(content, test.lookup)
			if publishIndex < 0 {
				t.Fatalf("release workflow is missing tag publication %q", test.publish)
			}
			if lookupIndex < 0 {
				t.Fatalf("release workflow is missing proxy lookup %q", test.lookup)
			}
			if publishIndex >= lookupIndex {
				t.Fatalf("%s proxy lookup occurs before tag publication", test.name)
			}
		})
	}
}

func TestPublicReleaseSeparatesCoreAndAdapterPublication(t *testing.T) {
	root := repositoryRoot(t)
	content := readFile(t, filepath.Join(root, ".github/workflows/release.yml"))
	coreNpmIndex := strings.Index(content, `./scripts/publish-npm-release.sh "$RELEASE_VERSION" dist/core core`)
	coreProxyIndex := strings.Index(content, `./scripts/wait-public-go-module.sh github.com/hsblabs/scrape-kdl "$RELEASE_TAG"`)
	delayIndex := strings.Index(content, "- name: Wait for adapter release window")
	playwrightNpmIndex := strings.Index(content, `./scripts/publish-npm-release.sh "$RELEASE_VERSION" dist/core playwright`)
	if coreNpmIndex < 0 || coreProxyIndex < 0 || delayIndex < 0 || playwrightNpmIndex < 0 {
		t.Fatal("release workflow is missing a core/adapter publication phase")
	}
	if !(coreNpmIndex < coreProxyIndex && coreProxyIndex < delayIndex && delayIndex < playwrightNpmIndex) {
		t.Fatalf("release phase order is incorrect: core npm=%d core proxy=%d delay=%d playwright npm=%d", coreNpmIndex, coreProxyIndex, delayIndex, playwrightNpmIndex)
	}
	for _, required := range []string{
		"GO_MODULE_WAIT_ATTEMPTS: 180",
		"GO_MODULE_WAIT_INTERVAL_SECONDS: 10",
		"ADAPTER_RELEASE_DELAY_SECONDS: 1800",
	} {
		if !strings.Contains(content, required) {
			t.Errorf("release workflow is missing %q", required)
		}
	}
}

func TestNpmReleaseWorkflowsPublishLocalArchives(t *testing.T) {
	root := repositoryRoot(t)
	tests := []struct {
		path     string
		required []string
		forbid   []string
	}{
		{
			path: "scripts/publish-npm-release.sh",
			required: []string{
				`npm publish "$archive" --tag "$dist_tag"`,
				`wait_for_npm_value "$package@$version" version "$version"`,
				`wait_for_npm_value "$package" "dist-tags.$dist_tag" "$version"`,
				`published_integrity="$(npm view "$package@$version" dist.integrity)"`,
			},
			forbid: []string{`npm publish "$archive" --access public`},
		},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, test.path))
			if err != nil {
				t.Fatal(err)
			}
			content := string(data)
			for _, required := range test.required {
				if !strings.Contains(content, required) {
					t.Errorf("%s is missing %q", test.path, required)
				}
			}
			for _, forbidden := range test.forbid {
				if strings.Contains(content, forbidden) {
					t.Errorf("%s contains forbidden %q", test.path, forbidden)
				}
			}
		})
	}
}
