package scrapekdl

// FetchMode is the validated acquisition mode of a Program.
type FetchMode string

const (
	FetchModeHTTP    FetchMode = "http"
	FetchModeBrowser FetchMode = "browser"
)

// SessionPolicy is the validated explicit-session policy of a Program.
type SessionPolicy string

const (
	SessionPolicyNone     SessionPolicy = "none"
	SessionPolicyOptional SessionPolicy = "optional"
	SessionPolicyRequired SessionPolicy = "required"
)

// SourceDescriptor contains immutable acquisition facts needed by a host.
type SourceDescriptor struct {
	FetchMode     FetchMode     `json:"fetchMode"`
	URLTemplate   string        `json:"urlTemplate"`
	SessionPolicy SessionPolicy `json:"sessionPolicy"`
}

// ProgramDescriptor is the small host-facing projection of a compiled Program.
type ProgramDescriptor struct {
	Source SourceDescriptor `json:"source"`
}

// Descriptor returns an immutable value projection of host-facing acquisition
// settings without exposing the internal Validated IR.
func (program *Program) Descriptor() ProgramDescriptor {
	return ProgramDescriptor{Source: SourceDescriptor{
		FetchMode:     FetchMode(program.extractor.Source.Fetch.Mode),
		URLTemplate:   program.extractor.Source.Fetch.URLTemplate.Raw,
		SessionPolicy: SessionPolicy(program.extractor.Source.SessionPolicy),
	}}
}
