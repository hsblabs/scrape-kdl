package executor

import (
	"context"
	"fmt"

	"github.com/hsblabs/scrape-kdl/internal/dom"
	"github.com/hsblabs/scrape-kdl/internal/ir"
)

// Prepared is an immutable execution plan for a validated extractor. It holds
// only static data that is safe to reuse across concurrent extractions.
type Prepared struct {
	extractor        *ir.Extractor
	mode             string
	selectors        map[string]dom.Selector
	postBindingError error
	snapshotError    error
}

// Prepare validates static runtime contracts and compiles reusable selectors.
// Dynamic options such as external transforms, sessions, policies, and
// cancellation remain extraction-local.
func Prepare(extractor *ir.Extractor) (*Prepared, error) {
	if extractor == nil {
		return nil, &ExecutionError{Code: "E_IR_INVALID", Message: "extractor IR is nil"}
	}
	if err := preflightExtractorStructure(extractor); err != nil {
		return nil, err
	}
	if err := preflightSourceStructure(extractor.Source); err != nil {
		return nil, err
	}
	transforms := newTransformRuntime(context.Background(), extractor, nil)
	if err := transforms.preflightExternalDeclarations(); err != nil {
		return nil, err
	}
	prepared := &Prepared{extractor: extractor, mode: extractor.Source.Fetch.Mode, selectors: map[string]dom.Selector{}}
	prepared.snapshotError = snapshotCompatibilityError(extractor)
	prepared.postBindingError = transforms.preflightStaticAfterBindings()
	if prepared.postBindingError == nil {
		prepared.postBindingError = preflightOutputStructure(extractor.Output)
	}
	if prepared.postBindingError != nil {
		return prepared, nil
	}
	switch prepared.mode {
	case "http":
		probe := &engine{extractor: extractor, selectors: prepared.selectors}
		if err := probe.preflightOutput(extractor.Output); err != nil {
			return nil, err
		}
	case "browser":
		if err := preflightBrowserWorkflow(extractor.Source.Workflow); err != nil {
			return nil, err
		}
		if err := preflightBrowserOutput(extractor.Output); err != nil {
			return nil, err
		}
		if prepared.snapshotError == nil {
			probe := &engine{extractor: extractor, selectors: prepared.selectors}
			if err := probe.preflightOutput(extractor.Output); err != nil {
				return nil, err
			}
		}
	default:
		return nil, &ExecutionError{Code: "E_IR_INVALID", Message: fmt.Sprintf("unknown fetch mode %q", prepared.mode), Path: "source.fetch.mode"}
	}
	prepared.postBindingError = preflightCapabilities(extractor)
	return prepared, nil
}

func (prepared *Prepared) Execute(ctx context.Context, inputs map[string]any, options Options) (*Result, error) {
	if prepared.mode == "browser" {
		return executeBrowserPrepared(ctx, prepared, inputs, options)
	}
	return executeHTTPPrepared(ctx, prepared, inputs, options)
}

func (prepared *Prepared) ExecuteHTML(ctx context.Context, html string, options Options) (*Result, error) {
	if prepared.mode != "http" {
		return nil, &ExecutionError{Code: "E_BROWSER_RUNTIME_MISSING", Message: fmt.Sprintf("HTTP runtime cannot execute fetch mode %q", prepared.mode)}
	}
	return executeHTMLPrepared(ctx, prepared, html, options)
}

func (prepared *Prepared) ExecuteSnapshot(ctx context.Context, html string, options Options) (*Result, error) {
	if err := executionContextError(ctx, "output"); err != nil {
		return nil, err
	}
	if prepared.snapshotError != nil {
		return nil, prepared.snapshotError
	}
	return executeSnapshotPrepared(ctx, prepared, html, options)
}

func snapshotCompatibilityError(extractor *ir.Extractor) error {
	if len(extractor.Source.Workflow) > 0 {
		return &ExecutionError{
			Code: "E_SNAPSHOT_UNSUPPORTED", Message: "offline snapshot execution cannot reproduce browser workflow",
			Path: "source.workflow",
		}
	}
	var inspect func(ir.OutputObject) error
	inspect = func(object ir.OutputObject) error {
		for _, member := range object.Members {
			switch typed := member.(type) {
			case ir.Field:
				if _, ok := typed.ValueSource.(ir.JavaScriptValueSource); ok {
					return &ExecutionError{
						Code: "E_SNAPSHOT_UNSUPPORTED", Message: "offline snapshot execution cannot evaluate JavaScript value sources",
						Path: typed.ID,
					}
				}
			case ir.Collection:
				if err := inspect(typed.Row); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return inspect(extractor.Output)
}
