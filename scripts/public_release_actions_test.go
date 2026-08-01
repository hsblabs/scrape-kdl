package scripts

import (
	"crypto/sha512"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWaitPublicGoModuleRetriesUntilTheProxyResolves(t *testing.T) {
	root := repositoryRoot(t)
	bin := t.TempDir()
	countFile := filepath.Join(t.TempDir(), "count")
	writeExecutable(t, filepath.Join(bin, "go"), `#!/bin/sh
count=0
if test -f "$COUNT_FILE"; then
  count="$(sed -n '1p' "$COUNT_FILE")"
fi
count=$((count + 1))
printf '%s\n' "$count" >"$COUNT_FILE"
if test "$count" -lt 2; then
  echo unavailable >&2
  exit 1
fi
printf '{"Path":"github.com/hsblabs/scrape-kdl","Version":"v1.2.3"}\n'
`)

	command := exec.Command("bash", filepath.Join(root, "scripts", "wait-public-go-module.sh"), coreModulePathForTest, "v1.2.3")
	command.Env = append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"COUNT_FILE="+countFile,
		"GO_MODULE_WAIT_ATTEMPTS=2",
		"GO_MODULE_WAIT_INTERVAL_SECONDS=0",
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("wait-public-go-module.sh: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), `"Version":"v1.2.3"`) {
		t.Fatalf("unexpected output: %s", output)
	}
	if count := strings.TrimSpace(readFile(t, countFile)); count != "2" {
		t.Fatalf("attempt count = %s; want 2", count)
	}
}

func TestPublishNpmReleaseVerifiesExistingArchivesWithoutRepublishing(t *testing.T) {
	root := repositoryRoot(t)
	artifacts := t.TempDir()
	version := "1.2.3"
	coreArchive := filepath.Join(artifacts, "hsblabs-scrape-kdl-"+version+".tgz")
	playwrightArchive := filepath.Join(artifacts, "hsblabs-scrape-kdl-playwright-"+version+".tgz")
	if err := os.WriteFile(coreArchive, []byte("core archive"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(playwrightArchive, []byte("playwright archive"), 0o644); err != nil {
		t.Fatal(err)
	}

	bin := t.TempDir()
	logFile := filepath.Join(t.TempDir(), "npm.log")
	writeExecutable(t, filepath.Join(bin, "npm"), `#!/bin/sh
printf '%s\n' "$*" >>"$NPM_LOG"
case "$1" in
  --version)
    echo 11.5.1
    ;;
  view)
    case "$3" in
      version) echo 1.2.3 ;;
      dist.integrity)
        case "$2" in
          @hsblabs/scrape-kdl@*) echo "$CORE_INTEGRITY" ;;
          @hsblabs/scrape-kdl-playwright@*) echo "$PLAYWRIGHT_INTEGRITY" ;;
        esac
        ;;
      dist-tags.latest) echo 1.2.3 ;;
    esac
    ;;
  publish)
    echo "unexpected npm publish" >&2
    exit 9
    ;;
  *)
    echo "unexpected npm command: $*" >&2
    exit 10
    ;;
esac
`)

	command := exec.Command("bash", filepath.Join(root, "scripts", "publish-npm-release.sh"), version, artifacts)
	command.Env = append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"NPM_LOG="+logFile,
		"CORE_INTEGRITY="+npmIntegrity([]byte("core archive")),
		"PLAYWRIGHT_INTEGRITY="+npmIntegrity([]byte("playwright archive")),
		"NPM_WAIT_ATTEMPTS=1",
		"NPM_WAIT_INTERVAL_SECONDS=0",
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("publish-npm-release.sh: %v\n%s", err, output)
	}
	log := readFile(t, logFile)
	if strings.Contains(log, "publish ") {
		t.Fatalf("already published packages were republished:\n%s", log)
	}
	for _, expected := range []string{
		"view @hsblabs/scrape-kdl@1.2.3 dist.integrity",
		"view @hsblabs/scrape-kdl-playwright@1.2.3 dist.integrity",
	} {
		if !strings.Contains(log, expected) {
			t.Errorf("npm log is missing %q:\n%s", expected, log)
		}
	}
}

func TestPublicReleaseActionsFailClosed(t *testing.T) {
	root := repositoryRoot(t)
	tests := []struct {
		path     string
		required []string
		forbid   []string
	}{
		{
			path: "scripts/publish-github-release.sh",
			required: []string{
				"release tag is not annotated",
				"remote release tag $tag points to",
				"GitHub Release $tag has unexpected asset",
				"cmp -s",
			},
			forbid: []string{"--clobber", "git tag -f", "git push --force"},
		},
		{
			path: "scripts/publish-npm-release.sh",
			required: []string{
				"npm-archive-integrity.mjs",
				"refusing to move npm",
				"dist.integrity",
			},
			forbid: []string{"NPM_TOKEN", "--access public"},
		},
		{
			path: "scripts/wait-public-go-module.sh",
			required: []string{
				"GOPROXY=https://proxy.golang.org",
				"GOSUMDB=sum.golang.org",
				"timed out waiting for",
			},
		},
	}

	for _, test := range tests {
		content := readFile(t, filepath.Join(root, test.path))
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
	}
}

const coreModulePathForTest = "github.com/hsblabs/scrape-kdl"

func npmIntegrity(data []byte) string {
	digest := sha512.Sum512(data)
	return "sha512-" + base64.StdEncoding.EncodeToString(digest[:])
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}
