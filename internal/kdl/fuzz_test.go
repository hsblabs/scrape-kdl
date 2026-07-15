package kdl

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func FuzzParseNeverPanics(f *testing.F) {
	data, err := os.ReadFile(filepath.Join("..", "..", "fixtures", "parser", "fuzz-seeds.json"))
	if err != nil {
		f.Fatal(err)
	}
	var seeds []string
	if err := json.Unmarshal(data, &seeds); err != nil {
		f.Fatal(err)
	}
	for _, seed := range seeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, source string) {
		_, _ = Parse("fuzz.kdl", []byte(source))
	})
}
