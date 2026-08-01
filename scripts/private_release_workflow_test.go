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
			path: ".github/workflows/release-npm.yml",
			required: []string{
				`REPOSITORY_VISIBILITY" != public`,
				"refusing to publish private release version publicly",
				"./scripts/validate-public-release-tag.sh core",
				"--access public",
				"id-token: write",
			},
			forbid: []string{"--access restricted", "NPM_TOKEN"},
		},
		{
			path: ".github/workflows/release.yml",
			required: []string{
				"./scripts/validate-public-release-tag.sh core",
				"--prerelease",
				"--latest=false",
			},
			forbid: []string{"./scripts/validate-private-release-tag.sh"},
		},
		{
			path: ".github/workflows/release-rod.yml",
			required: []string{
				"./scripts/validate-public-release-tag.sh rod",
				"--prerelease",
				"--latest=false",
			},
			forbid: []string{"./scripts/validate-private-release-tag.sh"},
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
	for _, path := range []string{
		".github/workflows/release-private-rod.yml",
		".github/workflows/release-rod.yml",
	} {
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		content := string(data)
		for _, required := range []string{"./scripts/build-rod-release.sh", "dist/*"} {
			if !strings.Contains(content, required) {
				t.Errorf("%s is missing %q", path, required)
			}
		}
	}
}

func TestNpmReleaseWorkflowsPublishLocalArchives(t *testing.T) {
	root := repositoryRoot(t)
	tests := []struct {
		path     string
		archives []string
		forbid   []string
	}{
		{
			path: ".github/workflows/release-npm-private.yml",
			archives: []string{
				`npm publish "./dist/hsblabs-scrape-kdl-$RELEASE_VERSION.tgz"`,
				`npm publish "./dist/hsblabs-scrape-kdl-playwright-$RELEASE_VERSION.tgz"`,
			},
		},
		{
			path: ".github/workflows/release-npm.yml",
			archives: []string{
				`npm publish "./$archive" --tag "$NPM_DIST_TAG"`,
			},
			forbid: []string{`npm publish "./$archive" --access public`},
		},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, test.path))
			if err != nil {
				t.Fatal(err)
			}
			content := string(data)
			for _, archive := range test.archives {
				if !strings.Contains(content, archive) {
					t.Errorf("%s is missing %q", test.path, archive)
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
