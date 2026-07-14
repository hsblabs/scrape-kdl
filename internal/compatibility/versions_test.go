package compatibility

import (
	"slices"
	"testing"
)

func TestDateIdentifiers(t *testing.T) {
	for _, value := range []string{"2026-07-15", "2000-02-29"} {
		if !IsDateIdentifier(value) {
			t.Fatalf("IsDateIdentifier(%q) = false", value)
		}
	}
	for _, value := range []string{"", "0.1", "2026-7-15", "2026-02-29", "2026-13-01"} {
		if IsDateIdentifier(value) {
			t.Fatalf("IsDateIdentifier(%q) = true", value)
		}
	}
}

func TestSupportedVersionRegistriesReturnCopies(t *testing.T) {
	languageVersions := SupportedLanguageVersions()
	irVersions := SupportedIRVersions()
	if !slices.Equal(languageVersions, []string{InitialLanguageVersion}) {
		t.Fatalf("language versions = %v", languageVersions)
	}
	if !slices.Equal(irVersions, []string{InitialIRVersion}) {
		t.Fatalf("IR versions = %v", irVersions)
	}
	languageVersions[0] = "mutated"
	irVersions[0] = "mutated"
	if !IsSupportedLanguageVersion(InitialLanguageVersion) || !IsSupportedIRVersion(InitialIRVersion) {
		t.Fatal("callers mutated the supported-version registries")
	}
}
