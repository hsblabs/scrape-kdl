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

func TestPublishNpmReleaseRejectsUnknownSelection(t *testing.T) {
	root := repositoryRoot(t)
	command := exec.Command(
		"bash",
		filepath.Join(root, "scripts", "publish-npm-release.sh"),
		"1.2.3",
		t.TempDir(),
		"adapter",
	)
	output, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "invalid npm release selection") {
		t.Fatalf("unknown selection error = %v\n%s", err, output)
	}
}

func TestPublishNpmReleaseRejectsExistingVersionWithoutExpectedDistTag(t *testing.T) {
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
  --version) echo 11.5.1 ;;
  view)
    case "$3" in
      version) echo 1.2.3 ;;
      dist.integrity) echo "$CORE_INTEGRITY" ;;
      dist-tags.latest) exit 0 ;;
    esac
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
		"NPM_WAIT_ATTEMPTS=1",
		"NPM_WAIT_INTERVAL_SECONDS=0",
	)
	output, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "trusted publishing cannot repair registry metadata") {
		t.Fatalf("missing dist-tag error = %v\n%s", err, output)
	}
	if log := readFile(t, logFile); strings.Contains(log, "dist-tag add") {
		t.Fatalf("missing dist-tag was mutated:\n%s", log)
	}
}

func TestPublishGitHubReleaseResumesMatchingStateAndRejectsDifferentAssets(t *testing.T) {
	sourceRoot := repositoryRoot(t)
	testRoot := filepath.Join(t.TempDir(), "repository")
	remote := filepath.Join(t.TempDir(), "remote.git")
	if err := os.MkdirAll(filepath.Join(testRoot, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"publish-github-release.sh",
		"validate-public-release-tag.sh",
		"validate-release-tag.sh",
	} {
		copyExecutable(t, filepath.Join(sourceRoot, "scripts", name), filepath.Join(testRoot, "scripts", name))
	}
	if err := os.WriteFile(filepath.Join(testRoot, "source.go"), []byte("package source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, "", "init", "--bare", remote)
	runTestGit(t, testRoot, "init")
	runTestGit(t, testRoot, "config", "user.email", "release@example.invalid")
	runTestGit(t, testRoot, "config", "user.name", "release test")
	runTestGit(t, testRoot, "add", ".")
	runTestGit(t, testRoot, "commit", "-m", "release source")
	runTestGit(t, testRoot, "remote", "add", "origin", remote)
	runTestGit(t, testRoot, "push", "-u", "origin", "HEAD")

	artifacts := t.TempDir()
	archive := filepath.Join(artifacts, "scrape-kdl_1.2.3_linux_amd64.tar.gz")
	if err := os.WriteFile(archive, []byte("inspected artifact"), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	state := t.TempDir()
	writeExecutable(t, filepath.Join(bin, "gh"), `#!/bin/sh
set -eu
command=$1
operation=$2
tag=$3
assets="$GH_STATE/assets"
case "$command:$operation" in
  release:view)
    test -f "$GH_STATE/release"
    case "$*" in
      *"--json isDraft"*) echo false ;;
      *"--json isPrerelease"*) echo false ;;
      *"--json assets"*) find "$assets" -type f -exec basename {} \; | sort ;;
    esac
    ;;
  release:create)
    shift 3
    mkdir -p "$assets"
    for argument in "$@"; do
      if test -f "$argument"; then
        cp "$argument" "$assets/"
      fi
    done
    touch "$GH_STATE/release"
    echo "create $*" >>"$GH_STATE/log"
    ;;
  release:upload)
    cp "$4" "$assets/"
    echo upload >>"$GH_STATE/log"
    ;;
  release:download)
    shift 3
    pattern=
    destination=
    while test "$#" -gt 0; do
      case "$1" in
        --pattern) pattern=$2; shift 2 ;;
        --dir) destination=$2; shift 2 ;;
        *) shift ;;
      esac
    done
    cp "$assets/$pattern" "$destination/"
    ;;
  *)
    echo "unexpected gh command: $*" >&2
    exit 2
    ;;
esac
`)

	run := func() ([]byte, error) {
		command := exec.Command(
			"bash",
			filepath.Join(testRoot, "scripts", "publish-github-release.sh"),
			"core",
			"v1.2.3",
			"HEAD",
			artifacts,
		)
		command.Dir = testRoot
		command.Env = append(os.Environ(),
			"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
			"GH_STATE="+state,
		)
		return command.CombinedOutput()
	}

	if output, err := run(); err != nil {
		t.Fatalf("first publication: %v\n%s", err, output)
	}
	if output, err := run(); err != nil {
		t.Fatalf("matching retry: %v\n%s", err, output)
	}
	if log := readFile(t, filepath.Join(state, "log")); strings.Count(log, "create") != 1 {
		t.Fatalf("GitHub Release create count is not one:\n%s", log)
	}
	log := readFile(t, filepath.Join(state, "log"))
	for _, expected := range []string{
		"--generate-notes",
		"--notes",
		"https://hsblabs.github.io/scrape-kdl/migrating-to-v1.md#go-api",
	} {
		if !strings.Contains(log, expected) {
			t.Errorf("GitHub Release create arguments are missing %q:\n%s", expected, log)
		}
	}
	remoteTag := strings.TrimSpace(runTestGit(t, testRoot, "ls-remote", "--tags", "origin", "refs/tags/v1.2.3^{}"))
	if remoteTag == "" {
		t.Fatal("annotated release tag was not pushed")
	}

	if err := os.WriteFile(archive, []byte("different artifact"), 0o644); err != nil {
		t.Fatal(err)
	}
	output, err := run()
	if err == nil || !strings.Contains(string(output), "asset differs from inspected artifact") {
		t.Fatalf("different retry error = %v\n%s", err, output)
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
			forbid: []string{"NPM_TOKEN", "--access public", "npm dist-tag add"},
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

func copyExecutable(t *testing.T, source, destination string) {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, data, 0o755); err != nil {
		t.Fatal(err)
	}
}

func runTestGit(t *testing.T, directory string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	if directory != "" {
		command.Dir = directory
	}
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output)
}
