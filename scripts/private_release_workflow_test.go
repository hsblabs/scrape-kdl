package scripts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrivateReleaseWorkflowsKeepPrivateAndPublicChannelsSeparate(t *testing.T) {
	root := repositoryRoot(t)
	tests := []struct {
		path     string
		required []string
		forbid   []string
	}{
		{
			path: ".github/workflows/release-private.yml",
			required: []string{
				"workflow_dispatch:",
				"PUBLISH_PRIVATE_GITHUB_RELEASE",
				`REPOSITORY_VISIBILITY" != private`,
				"./scripts/validate-private-release-tag.sh core",
				"make release-dist",
				"NPM_ACCESS=restricted",
				"gh release create",
				"--prerelease",
			},
			forbid: []string{`visibility == 'public'`, "NPM_ACCESS=public"},
		},
		{
			path: ".github/workflows/release-private-rod.yml",
			required: []string{
				"workflow_dispatch:",
				"PUBLISH_PRIVATE_ROD_RELEASE",
				`REPOSITORY_VISIBILITY" != private`,
				"./scripts/validate-private-release-tag.sh rod",
				"./scripts/build-rod-release.sh",
				"gh release create",
				"--prerelease",
			},
			forbid: []string{`visibility == 'public'`},
		},
		{
			path: ".github/workflows/release-npm-private.yml",
			required: []string{
				"workflow_dispatch:",
				"PUBLISH_PRIVATE_NPM",
				`REPOSITORY_VISIBILITY" != private`,
				"--access restricted",
				"--tag private",
				"id-token: write",
			},
			forbid: []string{"--access public", "NPM_TOKEN"},
		},
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
			path:     ".github/workflows/release-private-rod.yml",
			required: []string{"./scripts/build-rod-release.sh", "dist/*"},
		},
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

func TestPublicReleaseHasOneCallerVisibleWorkflow(t *testing.T) {
	root := repositoryRoot(t)
	for _, path := range []string{
		".github/workflows/release-npm.yml",
		".github/workflows/release-rod.yml",
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

func TestNpmReleaseWorkflowsPublishLocalArchives(t *testing.T) {
	root := repositoryRoot(t)
	tests := []struct {
		path     string
		required []string
		forbid   []string
	}{
		{
			path: ".github/workflows/release-npm-private.yml",
			required: []string{
				`npm publish "./dist/hsblabs-scrape-kdl-$RELEASE_VERSION.tgz"`,
				`npm publish "./dist/hsblabs-scrape-kdl-playwright-$RELEASE_VERSION.tgz"`,
			},
		},
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
