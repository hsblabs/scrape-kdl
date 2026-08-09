package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
	"golang.org/x/mod/sumdb/dirhash"
	modzip "golang.org/x/mod/zip"
)

const (
	coreModulePath = "github.com/hsblabs/scrape-kdl"
	rodModulePath  = "adapters/rod/go.mod"
	rodSumPath     = "adapters/rod/go.sum"
)

var (
	releaseExampleVersionPattern = regexp.MustCompile(`(?:v)?[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?`)
	currentReleaseLinePattern    = regexp.MustCompile(`^\s*Current (?:published candidate|stable release):`)
	goInstallLinePattern         = regexp.MustCompile(`^\s*go (?:install|get)\b`)
	npmInstallLinePattern        = regexp.MustCompile(`^\s*npm install\b`)
)

type moduleSums struct {
	zip   string
	goMod string
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	if len(args) != 3 || (args[0] != "prepare" && args[0] != "check") {
		return errors.New("usage: releaseplan <prepare|check> <vX.Y.Z> <revision>")
	}

	mode, version, revision := args[0], args[1], args[2]
	if err := validateVersion(version); err != nil {
		return err
	}
	root, err := repositoryRoot()
	if err != nil {
		return err
	}
	if err := checkReadmeVersion(root, version, revision); err != nil {
		return err
	}
	if mode == "prepare" {
		if err := requireCleanCoreTree(root, revision); err != nil {
			return err
		}
	}

	sums, err := calculateModuleSums(root, version, revision)
	if err != nil {
		return err
	}

	if mode == "prepare" {
		if err := prepareRodMetadata(root, version, sums); err != nil {
			return err
		}
	}
	if err := checkRodMetadata(root, version, sums); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "release plan: core=%s adapter=adapters/rod/%s revision=%s\n", version, version, revision)
	return nil
}

func checkReadmeVersion(root, version, revision string) error {
	data, err := gitFile(root, revision, "README.md")
	if err != nil {
		return err
	}
	return compareReadmeVersion(data, version)
}

func compareReadmeVersion(data []byte, version string) error {
	want := "v" + strings.TrimPrefix(version, "v")
	npmContinuation := false
	for _, line := range strings.Split(string(data), "\n") {
		npmLine := npmContinuation || npmInstallLinePattern.MatchString(line)
		if currentReleaseLinePattern.MatchString(line) || goInstallLinePattern.MatchString(line) || npmLine {
			for _, found := range releaseExampleVersionPattern.FindAllString(line, -1) {
				found = "v" + strings.TrimPrefix(found, "v")
				if found != want {
					return fmt.Errorf("README.md contains release version %s; want %s", found, want)
				}
			}
		}
		if npmLine {
			npmContinuation = strings.HasSuffix(strings.TrimSpace(line), `\`)
		} else {
			npmContinuation = false
		}
	}
	return nil
}

func validateVersion(version string) error {
	if !semver.IsValid(version) || strings.Contains(version, "-private.") {
		return fmt.Errorf("invalid public core version: %s", version)
	}
	return nil
}

func repositoryRoot() (string, error) {
	command := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("resolve repository root: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func requireCleanCoreTree(root, revision string) error {
	command := exec.Command(
		"git", "-C", root, "diff", "--quiet", revision, "--", ".",
		":(exclude)adapters/rod/go.mod",
		":(exclude)adapters/rod/go.sum",
	)
	if err := command.Run(); err != nil {
		return errors.New("commit release-facing changes before preparing go-rod metadata")
	}
	return nil
}

func calculateModuleSums(root, version, revision string) (moduleSums, error) {
	temporaryZip, err := os.CreateTemp("", "scrape-kdl-release-*.zip")
	if err != nil {
		return moduleSums{}, fmt.Errorf("create module zip: %w", err)
	}
	zipPath := temporaryZip.Name()
	defer os.Remove(zipPath)

	moduleVersion := module.Version{Path: coreModulePath, Version: version}
	if err := modzip.CreateFromVCS(temporaryZip, moduleVersion, root, revision, ""); err != nil {
		temporaryZip.Close()
		return moduleSums{}, fmt.Errorf("create module zip from %s: %w", revision, err)
	}
	if err := temporaryZip.Close(); err != nil {
		return moduleSums{}, fmt.Errorf("close module zip: %w", err)
	}

	zipSum, err := dirhash.HashZip(zipPath, dirhash.Hash1)
	if err != nil {
		return moduleSums{}, fmt.Errorf("hash module zip: %w", err)
	}
	goMod, err := gitFile(root, revision, "go.mod")
	if err != nil {
		return moduleSums{}, err
	}
	goModSum, err := dirhash.Hash1([]string{"go.mod"}, func(string) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(goMod)), nil
	})
	if err != nil {
		return moduleSums{}, fmt.Errorf("hash go.mod: %w", err)
	}

	return moduleSums{zip: zipSum, goMod: goModSum}, nil
}

func gitFile(root, revision, path string) ([]byte, error) {
	command := exec.Command("git", "-C", root, "show", revision+":"+path)
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("read %s from %s: %w", path, revision, err)
	}
	return output, nil
}

func prepareRodMetadata(root, version string, sums moduleSums) error {
	modulePath := filepath.Join(root, rodModulePath)
	moduleData, err := os.ReadFile(modulePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", rodModulePath, err)
	}
	parsed, err := modfile.Parse(rodModulePath, moduleData, nil)
	if err != nil {
		return fmt.Errorf("parse %s: %w", rodModulePath, err)
	}
	if err := parsed.AddRequire(coreModulePath, version); err != nil {
		return fmt.Errorf("set go-rod core dependency: %w", err)
	}
	parsed.Cleanup()
	formatted, err := parsed.Format()
	if err != nil {
		return fmt.Errorf("format %s: %w", rodModulePath, err)
	}
	if err := writeFileAtomic(modulePath, formatted, 0o644); err != nil {
		return err
	}

	sumPath := filepath.Join(root, rodSumPath)
	sumData, err := os.ReadFile(sumPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", rodSumPath, err)
	}
	lines := strings.Split(strings.TrimSpace(string(sumData)), "\n")
	kept := lines[:0]
	for _, line := range lines {
		if line != "" && !strings.HasPrefix(line, coreModulePath+" ") {
			kept = append(kept, line)
		}
	}
	kept = append(kept,
		fmt.Sprintf("%s %s %s", coreModulePath, version, sums.zip),
		fmt.Sprintf("%s %s/go.mod %s", coreModulePath, version, sums.goMod),
	)
	slices.Sort(kept)
	if err := writeFileAtomic(sumPath, []byte(strings.Join(kept, "\n")+"\n"), 0o644); err != nil {
		return err
	}
	return nil
}

func checkRodMetadata(root, version string, sums moduleSums) error {
	moduleData, err := os.ReadFile(filepath.Join(root, rodModulePath))
	if err != nil {
		return fmt.Errorf("read %s: %w", rodModulePath, err)
	}
	parsed, err := modfile.Parse(rodModulePath, moduleData, nil)
	if err != nil {
		return fmt.Errorf("parse %s: %w", rodModulePath, err)
	}
	var foundVersion string
	for _, requirement := range parsed.Require {
		if requirement.Mod.Path == coreModulePath {
			foundVersion = requirement.Mod.Version
		}
	}
	if foundVersion != version {
		return fmt.Errorf("%s requires %s; want %s", rodModulePath, foundVersion, version)
	}
	for _, replacement := range parsed.Replace {
		if replacement.Old.Path == coreModulePath {
			return fmt.Errorf("%s must not replace %s", rodModulePath, coreModulePath)
		}
	}

	want := map[string]string{
		version:             sums.zip,
		version + "/go.mod": sums.goMod,
	}
	found := make(map[string]string)
	sumData, err := os.ReadFile(filepath.Join(root, rodSumPath))
	if err != nil {
		return fmt.Errorf("read %s: %w", rodSumPath, err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(sumData)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 || fields[0] != coreModulePath {
			continue
		}
		found[fields[1]] = fields[2]
	}
	if len(found) != len(want) {
		return fmt.Errorf("%s must contain only the two anticipated %s sums", rodSumPath, coreModulePath)
	}
	for sumVersion, expected := range want {
		if found[sumVersion] != expected {
			return fmt.Errorf("%s has an unexpected %s %s checksum", rodSumPath, coreModulePath, sumVersion)
		}
	}
	return nil
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".releaseplan-*")
	if err != nil {
		return fmt.Errorf("create temporary %s: %w", path, err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return fmt.Errorf("set mode on %s: %w", path, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close %s: %w", path, err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}
