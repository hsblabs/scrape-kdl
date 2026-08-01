package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNestedModuleChangesDoNotChangeRootModuleSums(t *testing.T) {
	root := testRepository(t)
	before, err := calculateModuleSums(root, "v1.2.3", "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, root, "adapters/rod/adapter.go", "package rodadapter\n\nconst changed = true\n")
	runGit(t, root, "add", "adapters/rod/adapter.go")
	runGit(t, root, "commit", "-m", "change nested module")
	after, err := calculateModuleSums(root, "v1.2.3", "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	if before != after {
		t.Fatalf("nested module changed root sums: before=%+v after=%+v", before, after)
	}
}

func TestPrepareAndCheckRodMetadata(t *testing.T) {
	root := testRepository(t)
	sums, err := calculateModuleSums(root, "v1.2.3", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if err := prepareRodMetadata(root, "v1.2.3", sums); err != nil {
		t.Fatal(err)
	}
	if err := checkRodMetadata(root, "v1.2.3", sums); err != nil {
		t.Fatal(err)
	}

	moduleData := readTestFile(t, root, rodModulePath)
	if !strings.Contains(moduleData, coreModulePath+" v1.2.3") {
		t.Fatalf("go.mod was not updated:\n%s", moduleData)
	}
	sumData := readTestFile(t, root, rodSumPath)
	if strings.Contains(sumData, "v1.1.0") {
		t.Fatalf("go.sum retained the old core version:\n%s", sumData)
	}
	for _, expected := range []string{
		coreModulePath + " v1.2.3 " + sums.zip,
		coreModulePath + " v1.2.3/go.mod " + sums.goMod,
		"example.com/dependency v1.0.0 h1:dependency",
	} {
		if !strings.Contains(sumData, expected) {
			t.Errorf("go.sum is missing %q", expected)
		}
	}
}

func TestCheckRodMetadataRejectsDifferentChecksum(t *testing.T) {
	root := testRepository(t)
	sums, err := calculateModuleSums(root, "v1.2.3", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if err := prepareRodMetadata(root, "v1.2.3", sums); err != nil {
		t.Fatal(err)
	}
	sumPath := filepath.Join(root, rodSumPath)
	data := readTestFile(t, root, rodSumPath)
	writeTestFile(t, root, rodSumPath, strings.Replace(data, sums.zip, "h1:different", 1))

	err = checkRodMetadata(root, "v1.2.3", sums)
	if err == nil || !strings.Contains(err.Error(), "unexpected") {
		t.Fatalf("checkRodMetadata() error = %v", err)
	}
	if _, statErr := os.Stat(sumPath); statErr != nil {
		t.Fatal(statErr)
	}
}

func TestValidateVersion(t *testing.T) {
	for _, version := range []string{"v1.0.0", "v1.0.0-rc.3"} {
		if err := validateVersion(version); err != nil {
			t.Errorf("validateVersion(%q): %v", version, err)
		}
	}
	for _, version := range []string{"1.0.0", "v1.0.0-private.1", "latest"} {
		if err := validateVersion(version); err == nil {
			t.Errorf("validateVersion(%q) succeeded", version)
		}
	}
}

func TestRunRejectsUnknownMode(t *testing.T) {
	err := run([]string{"publish", "v1.0.0", "HEAD"}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("run() error = %v", err)
	}
}

func testRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, root, "go.mod", "module "+coreModulePath+"\n\ngo 1.26\n")
	writeTestFile(t, root, "library.go", "package scrapekdl\n")
	writeTestFile(t, root, rodModulePath, "module "+coreModulePath+"/adapters/rod\n\ngo 1.26\n\nrequire "+coreModulePath+" v1.1.0\n")
	writeTestFile(t, root, rodSumPath, coreModulePath+" v1.1.0 h1:old\n"+coreModulePath+" v1.1.0/go.mod h1:oldmod\nexample.com/dependency v1.0.0 h1:dependency\n")
	writeTestFile(t, root, "adapters/rod/adapter.go", "package rodadapter\n")
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "releaseplan@example.invalid")
	runGit(t, root, "config", "user.name", "releaseplan test")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "fixture")
	return root
}

func writeTestFile(t *testing.T, root, path, content string) {
	t.Helper()
	fullPath := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, root, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}
