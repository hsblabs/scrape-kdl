package kdl

import "testing"

func FuzzParseNeverPanics(f *testing.F) {
	seeds := []string{
		`extractor "x" version=1 { source "html" { fetch mode="http" url="https://example.com" } }`,
		`/- ignored "x"\nkept 0x10 #true #null`,
		`node #"raw"# key=#"value"#`,
		`node { child "value" }`,
		`/* nested /* block */ comment */ node`,
	}
	for _, seed := range seeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, source string) {
		_, _ = Parse("fuzz.kdl", []byte(source))
	})
}
