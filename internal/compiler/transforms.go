package compiler

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/hsblabs/scrape-kdl/internal/diagnostic"
	"github.com/hsblabs/scrape-kdl/internal/ir"
	"github.com/hsblabs/scrape-kdl/internal/kdl"
	"github.com/hsblabs/scrape-kdl/internal/typesys"
)

func (c *Compiler) compileReachableTransforms(root *loadedDocument) []ir.Transform {
	seen := map[string]bool{}
	out := []ir.Transform{}
	var visit func(*loadedDocument, string)
	visit = func(d *loadedDocument, origin string) {
		if d == nil {
			return
		}
		for _, alias := range d.importOrder {
			visit(d.imports[alias], "imported")
		}
		names := make([]string, 0, len(d.transformDecls))
		for name := range d.transformDecls {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			decl := d.transformDecls[name]
			if seen[decl.symbolID] {
				continue
			}
			seen[decl.symbolID] = true
			t := c.compileTransform(decl, origin)
			if t != nil {
				out = append(out, t)
			}
		}
	}
	for _, alias := range root.importOrder {
		visit(root.imports[alias], "imported")
	}
	// preserve local source order after imports
	for _, n := range root.root.Children {
		if n.Name != "transform" {
			continue
		}
		name, _ := stringArg(n, 0)
		decl := root.transformDecls[name]
		if decl == nil || seen[decl.symbolID] {
			continue
		}
		seen[decl.symbolID] = true
		if t := c.compileTransform(decl, "local"); t != nil {
			out = append(out, t)
		}
	}
	return out
}

func (c *Compiler) compileTransform(decl *transformDecl, origin string) ir.Transform {
	if decl == nil {
		return nil
	}
	if decl.compiled != nil {
		return withOrigin(decl.compiled, origin)
	}
	if decl.compiling {
		return nil
	}
	decl.compiling = true
	defer func() { decl.compiling = false }()
	var bodies []*kdl.Node
	for _, ch := range decl.node.Children {
		if ch.Name == "pipeline" || ch.Name == "match" || ch.Name == "external" {
			bodies = append(bodies, ch)
		} else {
			c.diags.Add("E_UNKNOWN_NODE", diagnostic.SeverityError, fmt.Sprintf("node %q is not allowed in transform", ch.Name), ch.Span, "transforms."+decl.name)
		}
	}
	if len(bodies) != 1 {
		c.diags.Add("E_DOCUMENT_ROOT", diagnostic.SeverityError, fmt.Sprintf("transform requires exactly one body; found %d", len(bodies)), decl.node.Span, "transforms."+decl.name)
		return nil
	}
	base := ir.TransformBase{SymbolID: decl.symbolID, Name: decl.name, Origin: origin, Input: decl.input, Output: decl.output, Span: decl.node.Span}
	body := bodies[0]
	switch body.Name {
	case "pipeline":
		validateNode(&c.diags, body, 0, 0, map[string]valueExpectation{}, "transforms."+decl.name+".pipeline")
		current := decl.input
		calls := []ir.TransformCall{}
		for i, ch := range body.Children {
			if ch.Name != "apply" {
				c.diags.Add("E_UNKNOWN_NODE", diagnostic.SeverityError, "pipeline only allows apply nodes", ch.Span, fmt.Sprintf("transforms.%s.pipeline[%d]", decl.name, i))
				continue
			}
			call, next := c.compileTransformCall(decl.doc, ch, current, fmt.Sprintf("transforms.%s.pipeline.calls[%d]", decl.name, i))
			calls = append(calls, call)
			current = next
		}
		if len(calls) == 0 {
			c.diags.Add("E_ARGUMENT_COUNT", diagnostic.SeverityError, "pipeline requires at least one apply", body.Span, "transforms."+decl.name+".pipeline")
		}
		if !typesys.IsAssignable(current, decl.output) {
			c.diags.Add("E_TRANSFORM_TYPE_MISMATCH", diagnostic.SeverityError, fmt.Sprintf("pipeline output %s is not assignable to declared output %s", current.String(), decl.output.String()), body.Span, "transforms."+decl.name)
		}
		t := ir.PipelineTransform{Kind: "pipeline", TransformBase: base, Calls: calls}
		decl.compiled = t
		return t
	case "match":
		validateNode(&c.diags, body, 0, 0, map[string]valueExpectation{}, "transforms."+decl.name+".match")
		if !typesys.IsScalar(decl.input) || !typesys.IsScalar(decl.output) {
			c.diags.Add("E_TRANSFORM_TYPE_MISMATCH", diagnostic.SeverityError, "match input and output must be scalar or nullable scalar", body.Span, "transforms."+decl.name)
		}
		cases := []ir.MatchCase{}
		var def json.RawMessage
		defaults := 0
		seen := map[string]bool{}
		for _, ch := range body.Children {
			switch ch.Name {
			case "case":
				validateNode(&c.diags, ch, 2, 2, map[string]valueExpectation{}, "transforms."+decl.name+".match")
				if len(ch.Arguments) < 2 {
					continue
				}
				when, _ := rawJSON(ch.Arguments[0])
				then, _ := rawJSON(ch.Arguments[1])
				key := string(when)
				if seen[key] {
					c.diags.Add("E_MATCH_DUPLICATE_CASE", diagnostic.SeverityError, "duplicate match case", ch.Span, "transforms."+decl.name+".match")
				}
				seen[key] = true
				if !isValueAssignable(ch.Arguments[0], decl.input) {
					c.diags.Add("E_TYPE_MISMATCH", diagnostic.SeverityError, "case input incompatible with transform input", ch.Span, "transforms."+decl.name+".match")
				}
				if !isValueAssignable(ch.Arguments[1], decl.output) {
					c.diags.Add("E_TYPE_MISMATCH", diagnostic.SeverityError, "case result incompatible with transform output", ch.Span, "transforms."+decl.name+".match")
				}
				cases = append(cases, ir.MatchCase{When: when, Then: then, Span: ch.Span})
			case "default":
				validateNode(&c.diags, ch, 1, 1, map[string]valueExpectation{}, "transforms."+decl.name+".match")
				defaults++
				if len(ch.Arguments) > 0 {
					def, _ = rawJSON(ch.Arguments[0])
					if !isValueAssignable(ch.Arguments[0], decl.output) {
						c.diags.Add("E_TYPE_MISMATCH", diagnostic.SeverityError, "default result incompatible with transform output", ch.Span, "transforms."+decl.name+".match")
					}
				}
			default:
				c.diags.Add("E_UNKNOWN_NODE", diagnostic.SeverityError, fmt.Sprintf("node %q is not allowed in match", ch.Name), ch.Span, "transforms."+decl.name+".match")
			}
		}
		if defaults != 1 {
			c.diags.Add("E_MATCH_DEFAULT", diagnostic.SeverityError, fmt.Sprintf("match requires exactly one default; found %d", defaults), body.Span, "transforms."+decl.name+".match")
		}
		t := ir.MatchTransform{Kind: "match", TransformBase: base, Cases: cases, Default: def}
		decl.compiled = t
		return t
	case "external":
		validateNode(&c.diags, body, 0, 0, map[string]valueExpectation{"symbol": expectString}, "transforms."+decl.name+".external")
		symbol, ok := stringProperty(body, "symbol")
		if !ok || symbol == "" {
			c.diags.Add("E_EXTERNAL_TRANSFORM_MISSING", diagnostic.SeverityError, "external transform requires non-empty symbol", body.Span, "transforms."+decl.name+".external")
		}
		c.capabilities["transform.external:"+symbol] = struct{}{}
		t := ir.ExternalTransform{Kind: "external", TransformBase: base, Symbol: symbol}
		decl.compiled = t
		return t
	default:
		return nil
	}
}

func withOrigin(t ir.Transform, origin string) ir.Transform {
	switch v := t.(type) {
	case ir.PipelineTransform:
		v.Origin = origin
		return v
	case ir.MatchTransform:
		v.Origin = origin
		return v
	case ir.ExternalTransform:
		v.Origin = origin
		return v
	default:
		return t
	}
}

func (c *Compiler) compileTransformCall(doc *loadedDocument, n *kdl.Node, input typesys.Type, path string) (ir.TransformCall, typesys.Type) {
	if len(n.Arguments) < 1 {
		c.diags.Add("E_ARGUMENT_COUNT", diagnostic.SeverityError, "apply requires a transform name", n.Span, path)
	}
	name, ok := stringArg(n, 0)
	if !ok {
		name = ""
		c.diags.Add("E_TRANSFORM_UNKNOWN", diagnostic.SeverityError, "apply target must be a string", n.Span, path)
	}
	call := ir.TransformCall{PositionalArguments: []json.RawMessage{}, NamedArguments: []ir.NamedArgument{}, Input: input, Output: input, Span: n.Span}
	for _, arg := range n.Arguments[1:] {
		raw, _ := rawJSON(arg)
		call.PositionalArguments = append(call.PositionalArguments, raw)
	}
	for _, prop := range n.Properties {
		raw, _ := rawJSON(prop.Value)
		call.NamedArguments = append(call.NamedArguments, ir.NamedArgument{Name: prop.Name, Value: raw})
	}
	if isBuiltin(name) {
		call.Target = ir.BuiltinTarget{Kind: "builtin", Name: name}
		allowed := builtinAllowedProperties(name)
		validateNode(&c.diags, n, 1, -1, allowed, path)
		validateRequiredProperties(&c.diags, n, builtinRequiredProperties(name), path)
		c.validateBuiltinArguments(name, input, n, path)
		if name != "assert-enum" && len(n.Arguments) > 1 {
			c.diags.Add("E_TRANSFORM_ARGUMENT", diagnostic.SeverityError, fmt.Sprintf("built-in %q does not accept positional arguments", name), n.Span, path)
		}
		if name == "assert-enum" && len(n.Arguments) < 2 {
			c.diags.Add("E_TRANSFORM_ARGUMENT", diagnostic.SeverityError, "assert-enum requires at least one allowed value", n.Span, path)
		}
		out, err := applyBuiltin(name, input, n)
		if err != nil {
			c.diags.Add("E_TRANSFORM_TYPE_MISMATCH", diagnostic.SeverityError, fmt.Sprintf("%s: %v", name, err), n.Span, path)
			out = input
		}
		call.Output = out
		return call, out
	}
	decl := c.resolveTransform(doc, name)
	if decl == nil {
		c.diags.Add("E_TRANSFORM_UNKNOWN", diagnostic.SeverityError, fmt.Sprintf("unknown transform %q", name), n.Span, path)
		call.Target = ir.DeclaredTarget{Kind: "declared", SymbolID: "unresolved:" + name}
		return call, input
	}
	call.Target = ir.DeclaredTarget{Kind: "declared", SymbolID: decl.symbolID}
	if len(n.Arguments) > 1 || len(n.Properties) > 0 {
		c.diags.Add("E_TRANSFORM_ARGUMENT", diagnostic.SeverityError, "declared transforms accept no call arguments or properties", n.Span, path)
	}
	if !typesys.IsAssignable(input, decl.input) {
		c.diags.Add("E_TRANSFORM_TYPE_MISMATCH", diagnostic.SeverityError, fmt.Sprintf("transform %q requires %s, got %s", name, decl.input.String(), input.String()), n.Span, path)
	}
	call.Output = decl.output
	return call, decl.output
}

func (c *Compiler) resolveTransform(doc *loadedDocument, name string) *transformDecl {
	if strings.Contains(name, ".") {
		parts := strings.Split(name, ".")
		if len(parts) != 2 {
			return nil
		}
		imp := doc.imports[parts[0]]
		if imp == nil {
			return nil
		}
		return imp.transformDecls[parts[1]]
	}
	return doc.transformDecls[name]
}
