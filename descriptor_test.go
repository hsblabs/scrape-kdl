package scrapekdl_test

import (
	"context"
	"testing"

	scrapekdl "github.com/hsblabs/scrape-kdl"
)

func TestProgramDescriptorReturnsImmutableAcquisitionFacts(t *testing.T) {
	program, diagnostics := compileFile(t, context.Background(), "fixtures/valid/race-detail.kdl")
	if diagnostics.HasErrors() {
		t.Fatalf("compile diagnostics = %#v", diagnostics)
	}
	descriptor := program.Descriptor()
	want := scrapekdl.SourceDescriptor{
		FetchMode:     scrapekdl.FetchModeBrowser,
		URLTemplate:   "https://example.com/race/{race_id}",
		SessionPolicy: scrapekdl.SessionPolicyOptional,
	}
	if descriptor.Source != want {
		t.Fatalf("descriptor source = %#v, want %#v", descriptor.Source, want)
	}

	descriptor.Source.URLTemplate = "mutated"
	if program.Descriptor().Source != want {
		t.Fatalf("descriptor mutation changed program: %#v", program.Descriptor())
	}
}
