package scripts

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyRodLeavesModuleMetadataUntouched(t *testing.T) {
	root := repositoryRoot(t)
	moduleFile := filepath.Join(root, "adapters", "rod", "go.mod")
	wantModule := readFile(t, moduleFile)

	logFile, run := fakeGoRunner(t, root, "verify-rod.sh", false)
	if output, err := run(); err != nil {
		t.Fatalf("verify-rod.sh failed: %v\n%s", err, output)
	}

	if got := readFile(t, moduleFile); got != wantModule {
		t.Fatal("verify-rod.sh changed adapters/rod/go.mod")
	}
	log := readFile(t, logFile)
	for _, command := range []string{"mod edit", "test", "vet"} {
		if !logContainsCommand(log, command) {
			t.Errorf("fake go log does not contain %q:\n%s", command, log)
		}
	}
	assertTemporaryModfilesRemoved(t, log)
}

func TestVerifyRodCleansUpAfterFailure(t *testing.T) {
	root := repositoryRoot(t)
	moduleFile := filepath.Join(root, "adapters", "rod", "go.mod")
	wantModule := readFile(t, moduleFile)

	logFile, run := fakeGoRunner(t, root, "verify-rod.sh", true)
	output, err := run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 42 {
		t.Fatalf("verify-rod.sh error = %v, want exit status 42\n%s", err, output)
	}

	if got := readFile(t, moduleFile); got != wantModule {
		t.Fatal("verify-rod.sh changed adapters/rod/go.mod after failure")
	}
	log := readFile(t, logFile)
	if strings.Contains(log, "vet ./...|") {
		t.Fatalf("verify-rod.sh ran go vet after go test failed:\n%s", log)
	}
	assertTemporaryModfilesRemoved(t, log)
}

func TestVerifyRodContractLeavesModuleMetadataUntouched(t *testing.T) {
	root := repositoryRoot(t)
	moduleFile := filepath.Join(root, "adapters", "rod", "go.mod")
	wantModule := readFile(t, moduleFile)

	logFile, run := fakeGoRunner(t, root, "verify-rod-contract.sh", false)
	if output, err := run(); err != nil {
		t.Fatalf("verify-rod-contract.sh failed: %v\n%s", err, output)
	}
	if got := readFile(t, moduleFile); got != wantModule {
		t.Fatal("verify-rod-contract.sh changed adapters/rod/go.mod")
	}
	log := readFile(t, logFile)
	for _, command := range []string{"mod edit", "test", "vet"} {
		if !logContainsCommand(log, command) {
			t.Errorf("fake go log does not contain %q:\n%s", command, log)
		}
	}
	assertTemporaryModfilesRemoved(t, log)
}

func TestVerifyRodContractCleansUpAfterFailure(t *testing.T) {
	root := repositoryRoot(t)
	moduleFile := filepath.Join(root, "adapters", "rod", "go.mod")
	wantModule := readFile(t, moduleFile)

	logFile, run := fakeGoRunner(t, root, "verify-rod-contract.sh", true)
	output, err := run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 42 {
		t.Fatalf("verify-rod-contract.sh error = %v, want exit status 42\n%s", err, output)
	}
	if got := readFile(t, moduleFile); got != wantModule {
		t.Fatal("verify-rod-contract.sh changed adapters/rod/go.mod after failure")
	}
	log := readFile(t, logFile)
	if strings.Contains(log, "vet -modfile=") {
		t.Fatalf("verify-rod-contract.sh ran go vet after go test failed:\n%s", log)
	}
	assertTemporaryModfilesRemoved(t, log)
}

func logContainsCommand(log, command string) bool {
	for _, line := range strings.Split(log, "\n") {
		arguments, _, _ := strings.Cut(line, "|")
		if arguments == command || strings.HasPrefix(arguments, command+" ") {
			return true
		}
	}
	return false
}

func fakeGoRunner(t *testing.T, root, scriptName string, failTest bool) (string, func() (string, error)) {
	t.Helper()
	tmp := t.TempDir()
	logFile := filepath.Join(tmp, "go.log")
	fakeGo := filepath.Join(tmp, "go")
	script := `#!/bin/sh
set -eu
printf '%s|%s\n' "$*" "${GOWORK-}" >> "$VERIFY_ROD_LOG"
modfile=
for argument in "$@"; do
  case "$argument" in
    -modfile=*) modfile=${argument#-modfile=} ;;
  esac
done
if [ "${1-} ${2-}" = "mod edit" ]; then
	if [ -z "$modfile" ]; then
		printf '\nreplace example.invalid/module => /tmp/local\n' >> "$VERIFY_ROD_MODULE/go.mod"
	fi
fi
if grep -q '^replace ' "$VERIFY_ROD_MODULE/go.mod"; then
  exit 91
fi
if [ "${1-}" = "test" ] && [ "${VERIFY_ROD_FAIL_TEST-}" = "1" ]; then
  exit 42
fi
`
	if err := os.WriteFile(fakeGo, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	run := func() (string, error) {
		command := exec.Command("bash", filepath.Join(root, "scripts", scriptName))
		command.Dir = root
		command.Env = append(os.Environ(),
			"PATH="+tmp+string(os.PathListSeparator)+os.Getenv("PATH"),
			"VERIFY_ROD_LOG="+logFile,
			"VERIFY_ROD_MODULE="+filepath.Join(root, "adapters", "rod"),
			fmt.Sprintf("VERIFY_ROD_FAIL_TEST=%d", map[bool]int{false: 0, true: 1}[failTest]),
		)
		output, err := command.CombinedOutput()
		return string(output), err
	}
	return logFile, run
}

func assertTemporaryModfilesRemoved(t *testing.T, log string) {
	t.Helper()
	for _, line := range strings.Split(strings.TrimSpace(log), "\n") {
		arguments, _, _ := strings.Cut(line, "|")
		for _, argument := range strings.Fields(arguments) {
			if !strings.HasPrefix(argument, "-modfile=") {
				continue
			}
			modfile := strings.TrimPrefix(argument, "-modfile=")
			if _, err := os.Stat(modfile); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("temporary modfile still exists after rod verification: %s", modfile)
			}
		}
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Dir(workingDirectory)
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}
