package scripts

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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
}

func TestBuildReleaseCleansStageAfterBuildFailure(t *testing.T) {
	root := repositoryRoot(t)
	tmp, output, err := runBuildRelease(t, root, true)
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 42 {
		t.Fatalf("build-release.sh error = %v, want exit status 42\n%s", err, output)
	}
	assertDirectoryEmpty(t, filepath.Join(tmp, "stages"))
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
	command := exec.Command("bash", filepath.Join(root, "scripts", "build-release.sh"), "v0.1.0", filepath.Join(tmp, "dist"))
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
