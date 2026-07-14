package compatibility

import "time"

const (
	InitialLanguageVersion = "2026-07-15"
	InitialIRVersion       = "2026-07-15"
)

var (
	supportedLanguageVersions = []string{InitialLanguageVersion}
	supportedIRVersions       = []string{InitialIRVersion}
)

func IsDateIdentifier(value string) bool {
	if len(value) != len("2006-01-02") {
		return false
	}
	parsed, err := time.Parse("2006-01-02", value)
	return err == nil && parsed.Format("2006-01-02") == value
}

func IsSupportedLanguageVersion(value string) bool {
	return contains(supportedLanguageVersions, value)
}

func IsSupportedIRVersion(value string) bool {
	return contains(supportedIRVersions, value)
}

func SupportedLanguageVersions() []string {
	return append([]string(nil), supportedLanguageVersions...)
}

func SupportedIRVersions() []string {
	return append([]string(nil), supportedIRVersions...)
}

func contains(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
