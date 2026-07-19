package scripts

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildReleaseCleansStageAfterSuccess(t *testing.T) {
	root := repositoryRoot(t)
	tmp, output, err := runBuildRelease(t, root, false)
	if err != nil {
		t.Fatalf("build-release.sh failed: %v\n%s", err, output)
	}
	assertDirectoryEmpty(t, filepath.Join(tmp, "stages"))
	for _, name := range []string{"scrape-kdl_0.1.0_linux_amd64.tar.gz", "checksums.txt"} {
		if _, err := os.Stat(filepath.Join(tmp, "dist", name)); err != nil {
			t.Fatalf("release output %s: %v", name, err)
		}
	}
	assertArchiveContains(t, filepath.Join(tmp, "dist", "scrape-kdl_0.1.0_linux_amd64.tar.gz"), "scrape-kdl", "LICENSE", "NOTICE", "README.md")
}

func TestBuildReleaseCleansStageAfterBuildFailure(t *testing.T) {
	root := repositoryRoot(t)
	tmp, output, err := runBuildRelease(t, root, true)
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 42 {
		t.Fatalf("build-release.sh error = %v, want exit status 42\n%s", err, output)
	}
	if contents, readErr := os.ReadFile(filepath.Join(tmp, "dist", "sentinel")); readErr != nil || string(contents) != "preserve" {
		t.Fatalf("existing release output was not preserved after failure: %q, %v", contents, readErr)
	}
	assertDirectoryEmpty(t, filepath.Join(tmp, "stages"))
}

func TestResolveReleaseOutputRejectsRepositoryAliases(t *testing.T) {
	root := repositoryRoot(t)
	resolver := filepath.Join(root, "scripts", "resolve-release-output.sh")
	for _, output := range []string{
		root,
		root + string(os.PathSeparator),
		root + string(os.PathSeparator) + ".",
		root + string(os.PathSeparator) + "unused" + string(os.PathSeparator) + "..",
	} {
		t.Run(output, func(t *testing.T) {
			command := exec.Command("bash", resolver, root, output)
			combined, err := command.CombinedOutput()
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || !strings.Contains(string(combined), "refusing to replace unsafe release output directory") {
				t.Fatalf("resolve-release-output.sh error = %v, output = %q", err, combined)
			}
		})
	}
}

func TestResolveReleaseOutputNormalizesParent(t *testing.T) {
	root := t.TempDir()
	resolver := filepath.Join(repositoryRoot(t), "scripts", "resolve-release-output.sh")
	output := root + string(os.PathSeparator) + "nested" + string(os.PathSeparator) + ".." + string(os.PathSeparator) + "dist"
	command := exec.Command("bash", resolver, root, output)
	combined, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("resolve-release-output.sh failed: %v\n%s", err, combined)
	}
	physicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(physicalRoot, "dist")
	if got := strings.TrimSpace(string(combined)); got != want {
		t.Fatalf("resolved output = %q, want %q", got, want)
	}
}

func runBuildRelease(t *testing.T, root string, fail bool) (string, string, error) {
	t.Helper()
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "bin")
	stages := filepath.Join(tmp, "stages")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(stages, 0o755); err != nil {
		t.Fatal(err)
	}
	dist := filepath.Join(tmp, "dist")
	if fail {
		if err := os.MkdirAll(dist, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dist, "sentinel"), []byte("preserve"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fakeGo := `#!/bin/sh
set -eu
if [ "${BUILD_RELEASE_FAIL-}" = 1 ]; then
  exit 42
fi
output=
while [ "$#" -gt 0 ]; do
  if [ "$1" = -o ]; then
    output=$2
    shift 2
  else
    shift
  fi
done
test -n "$output"
printf 'fake binary' > "$output"
`
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(fakeGo), 0o755); err != nil {
		t.Fatal(err)
	}
	failValue := "0"
	if fail {
		failValue = "1"
	}
	command := exec.Command("bash", filepath.Join(root, "scripts", "build-release.sh"), "v0.1.0", dist)
	command.Dir = root
	command.Env = append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"TMPDIR="+stages,
		"GITHUB_SHA=test-commit",
		"SCRAPE_KDL_RELEASE_TARGETS=linux/amd64",
		"BUILD_RELEASE_FAIL="+failValue,
	)
	output, err := command.CombinedOutput()
	return tmp, string(output), err
}

func assertArchiveContains(t *testing.T, path string, expected ...string) {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer gzipReader.Close()
	found := map[string]bool{}
	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		found[header.Name] = true
	}
	for _, name := range expected {
		if !found[name] {
			t.Errorf("archive %s is missing %s", path, name)
		}
	}
}

func assertDirectoryEmpty(t *testing.T, path string) {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary stage directory contains %v", entries)
	}
}
