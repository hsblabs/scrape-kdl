package scripts

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteReleaseChecksumsPrefersSHA256Sum(t *testing.T) {
	log := runChecksumScript(t, map[string]string{
		"sha256sum": `#!/bin/sh
printf 'sha256sum:%s\n' "$*" >> "$CHECKSUM_LOG"
printf 'linux-digest  %s\n' "$1"
`,
		"shasum": `#!/bin/sh
printf 'shasum:%s\n' "$*" >> "$CHECKSUM_LOG"
exit 99
`,
	}, true)
	if !strings.Contains(log, "sha256sum:./archive.tar.gz") || strings.Contains(log, "shasum:") {
		t.Fatalf("checksum command log = %q", log)
	}
}

func TestWriteReleaseChecksumsFallsBackToShasum(t *testing.T) {
	log := runChecksumScript(t, map[string]string{
		"shasum": `#!/bin/sh
printf 'shasum:%s\n' "$*" >> "$CHECKSUM_LOG"
test "$1" = -a
test "$2" = 256
shift 2
printf 'macos-digest  %s\n' "$1"
`,
	}, true)
	if !strings.Contains(log, "shasum:-a 256 ./archive.tar.gz") {
		t.Fatalf("checksum command log = %q", log)
	}
}

func TestWriteReleaseChecksumsDoesNotHashItself(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "archive.tar.gz"), []byte("archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "checksums.txt"), []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := repositoryRoot(t)
	command := exec.Command("/bin/bash", filepath.Join(root, "scripts", "write-release-checksums.sh"), tmp)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("write-release-checksums.sh failed: %v\n%s", err, output)
	}
	contents, err := os.ReadFile(filepath.Join(tmp, "checksums.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents), "checksums.txt") || !strings.Contains(string(contents), "archive.tar.gz") {
		t.Fatalf("checksums.txt = %q", contents)
	}
}

func TestWriteReleaseChecksumsFailsWithoutUtility(t *testing.T) {
	runChecksumScript(t, nil, false)
}

func runChecksumScript(t *testing.T, utilities map[string]string, wantSuccess bool) string {
	t.Helper()
	root := repositoryRoot(t)
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "bin")
	dist := filepath.Join(tmp, "dist")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dist, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "archive.tar.gz"), []byte("archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, contents := range utilities {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(contents), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	logFile := filepath.Join(tmp, "commands.log")
	command := exec.Command("/bin/bash", filepath.Join(root, "scripts", "write-release-checksums.sh"), dist)
	command.Env = append(os.Environ(), "PATH="+bin, "CHECKSUM_LOG="+logFile)
	output, err := command.CombinedOutput()
	if wantSuccess {
		if err != nil {
			t.Fatalf("write-release-checksums.sh failed: %v\n%s", err, output)
		}
		checksums, readErr := os.ReadFile(filepath.Join(dist, "checksums.txt"))
		if readErr != nil || !strings.Contains(string(checksums), "./archive.tar.gz") {
			t.Fatalf("checksums.txt = %q, %v", checksums, readErr)
		}
	} else {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || !strings.Contains(string(output), "no SHA-256 checksum utility found") {
			t.Fatalf("error = %v, output = %q", err, output)
		}
		if _, statErr := os.Stat(filepath.Join(dist, "checksums.txt")); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("checksums.txt exists after failure: %v", statErr)
		}
	}
	log, err := os.ReadFile(logFile)
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(log)
}
