package compiler

import (
	"fmt"
	"strings"

	"github.com/hsblabs/scrape-kdl/internal/diagnostic"
	"github.com/hsblabs/scrape-kdl/internal/ir"
	"github.com/hsblabs/scrape-kdl/internal/kdl"
	"github.com/hsblabs/scrape-kdl/internal/typesys"
)

func (c *Compiler) compileOutputObject(owner *loadedDocument, nodes []*kdl.Node, path string, scopeDoc *loadedDocument) ir.OutputObject {
	out := ir.OutputObject{Kind: "object", Members: []ir.OutputMember{}}
	seen := map[string]bool{}
	for _, n := range nodes {
		if n.Name != "field" && n.Name != "collection" {
			if path == "output" && (n.Name == "source" || n.Name == "input" || n.Name == "transform") {
				continue
			}
			if path == "output" {
				c.diags.Add("E_UNKNOWN_NODE", diagnostic.SeverityError, fmt.Sprintf("node %q is not allowed in extractor", n.Name), n.Span, path)
			}
			continue
		}
		name, _ := stringArg(n, 0)
		memberPath := path + "." + name
		if seen[name] {
			c.diags.Add("E_DUPLICATE_SYMBOL", diagnostic.SeverityError, fmt.Sprintf("duplicate output member %q", name), n.Span, memberPath)
		}
		seen[name] = true
		if n.Name == "field" {
			out.Members = append(out.Members, c.compileField(owner, n, memberPath, scopeDoc))
		} else {
			out.Members = append(out.Members, c.compileCollection(owner, n, memberPath, scopeDoc))
		}
	}
	return out
}

func (c *Compiler) compileField(owner *loadedDocument, n *kdl.Node, path string, scopeDoc *loadedDocument) ir.Field {
	validateNode(&c.diags, n, 1, 1, map[string]valueExpectation{"type": expectString, "required": expectBool, "default": expectScalar}, path)
	name, ok := stringArg(n, 0)
	if !ok {
		name = "invalid"
		c.diags.Add("E_TYPE_MISMATCH", diagnostic.SeverityError, "field name must be a string", n.Span, path)
	}
	validateIdentifier(&c.diags, name, n.Span, false, path)
	typeStr, ok := stringProperty(n, "type")
	var success typesys.Type
	if !ok {
		c.diags.Add("E_FIELD_TYPE_REQUIRED", diagnostic.SeverityError, "field requires string property type", n.Span, path)
		success = typesys.Primitive("unknown")
	} else {
		var err error
		success, err = typesys.Parse(typeStr)
		if err != nil {
			c.diags.Add("E_TYPE_UNKNOWN", diagnostic.SeverityError, err.Error(), n.Span, path)
			success = typesys.Primitive("unknown")
		}
	}
	required, ok := boolProperty(n, "required", false)
	if !ok {
		required = false
	}
	field := ir.Field{Kind: "field", ID: path, Name: name, SuccessfulType: success, Required: required, Transforms: []ir.TransformCall{}, Span: n.Span}
	if def, has := n.Property("default"); has {
		raw, _ := rawJSONPtr(def)
		field.Default = raw
		if !isValueAssignable(def, success) {
			c.diags.Add("E_DEFAULT_INVALID", diagnostic.SeverityError, fmt.Sprintf("default type %s is not assignable to %s", valueType(def).String(), success.String()), def.Span, path)
		}
	}
	if required && field.Default != nil {
		c.diags.Add("E_DEFAULT_INVALID", diagnostic.SeverityError, "required field must not declare default", n.Span, path)
	}

	var selects, values, evals, onErrors, applies []*kdl.Node
	for _, ch := range n.Children {
		switch ch.Name {
		case "select":
			selects = append(selects, ch)
		case "value":
			values = append(values, ch)
		case "evaluate-js":
			evals = append(evals, ch)
		case "on-error":
			onErrors = append(onErrors, ch)
		case "apply":
			applies = append(applies, ch)
		default:
			c.diags.Add("E_UNKNOWN_NODE", diagnostic.SeverityError, fmt.Sprintf("node %q is not allowed in field", ch.Name), ch.Span, path)
		}
	}
	if len(selects) > 1 {
		c.diags.Add("E_ARGUMENT_COUNT", diagnostic.SeverityError, "field allows at most one select", selects[1].Span, path+".selection")
	}
	if len(values)+len(evals) == 0 {
		c.diags.Add("E_VALUE_SOURCE_MISSING", diagnostic.SeverityError, "field requires value or evaluate-js", n.Span, path)
	}
	if len(values)+len(evals) > 1 {
		c.diags.Add("E_VALUE_SOURCE_MULTIPLE", diagnostic.SeverityError, "field has multiple value sources", n.Span, path)
	}
	if len(onErrors) > 1 {
		c.diags.Add("E_ARGUMENT_COUNT", diagnostic.SeverityError, "field allows at most one on-error", onErrors[1].Span, path+".onError")
	}

	if len(selects) > 0 {
		selNode := selects[0]
		validateNode(&c.diags, selNode, 1, 1, map[string]valueExpectation{"match": expectString}, path+".selection")
		sel, _ := stringArg(selNode, 0)
		c.checkSelector(sel, selNode, path+".selection")
		match, _ := stringProperty(selNode, "match")
		if match == "" {
			match = "one"
		}
		if match != "one" && match != "first" {
			c.diags.Add("E_TYPE_MISMATCH", diagnostic.SeverityError, "select match must be one or first", selNode.Span, path+".selection")
		}
		field.Selection = &ir.FieldSelection{Selector: sel, Match: match, Span: selNode.Span}
	}

	rawType := typesys.Primitive("string")
	if len(values) > 0 {
		v := values[0]
		kind, _ := stringArg(v, 0)
		allowed := map[string]valueExpectation{}
		if kind == "attr" {
			allowed["name"] = expectString
		}
		validateNode(&c.diags, v, 1, 1, allowed, path+".valueSource")
		if field.Selection == nil {
			c.diags.Add("E_SELECTOR_REQUIRED", diagnostic.SeverityError, "value source requires select", v.Span, path+".selection")
		}
		switch kind {
		case "text":
			field.ValueSource = ir.TextValueSource{Kind: "text", RawType: rawType, Span: v.Span}
			c.addReadCapability(owner, "text")
		case "html":
			field.ValueSource = ir.HTMLValueSource{Kind: "html", RawType: rawType, Span: v.Span}
			c.addReadCapability(owner, "html")
		case "attr":
			attr, ok := stringProperty(v, "name")
			if !ok || attr == "" {
				c.diags.Add("E_ATTRIBUTE_NAME_REQUIRED", diagnostic.SeverityError, "attr value requires non-empty name", v.Span, path+".valueSource")
			}
			field.ValueSource = ir.AttributeValueSource{Kind: "attribute", Name: attr, RawType: rawType, Span: v.Span}
			c.addReadCapability(owner, "attr")
		default:
			c.diags.Add("E_TYPE_MISMATCH", diagnostic.SeverityError, "value kind must be text, html, or attr", v.Span, path+".valueSource")
			field.ValueSource = ir.TextValueSource{Kind: "text", RawType: rawType, Span: v.Span}
		}
	}
	if len(evals) > 0 {
		e := evals[0]
		validateNode(&c.diags, e, 1, 1, map[string]valueExpectation{"scope": expectString, "returns": expectString, "timeout-ms": expectNonNegativeInt}, path+".valueSource")
		src, _ := stringArg(e, 0)
		scope, _ := stringProperty(e, "scope")
		returnsS, rok := stringProperty(e, "returns")
		if scope != "document" && scope != "current" {
			c.diags.Add("E_TYPE_MISMATCH", diagnostic.SeverityError, "evaluate-js scope must be document or current", e.Span, path+".valueSource")
		}
		returns := typesys.Primitive("unknown")
		if !rok {
			c.diags.Add("E_TYPE_UNKNOWN", diagnostic.SeverityError, "evaluate-js requires returns type", e.Span, path+".valueSource")
		} else if t, err := typesys.Parse(returnsS); err != nil {
			c.diags.Add("E_TYPE_UNKNOWN", diagnostic.SeverityError, err.Error(), e.Span, path+".valueSource")
		} else {
			returns = t
		}
		if scope == "document" && field.Selection != nil {
			c.diags.Add("E_SELECTOR_FORBIDDEN", diagnostic.SeverityError, "document-scoped evaluate-js forbids select", e.Span, path+".selection")
		}
		if scope == "current" && field.Selection == nil && !strings.Contains(path, "[]") {
			c.diags.Add("E_CURRENT_SCOPE_UNAVAILABLE", diagnostic.SeverityError, "top-level current scope requires select", e.Span, path+".valueSource")
		}
		field.ValueSource = ir.JavaScriptValueSource{Kind: "javascript", Scope: scope, Source: src, Returns: returns, TimeoutMS: positiveIntPtr(c, e, "timeout-ms", path+".valueSource"), Span: e.Span}
		rawType = returns
		c.capabilities["browser.evaluate-js"] = struct{}{}
		if field.Selection != nil && ownerSourceMode(owner) == "browser" {
			c.capabilities["browser.query"] = struct{}{}
		}
		c.jsPresent = true
		if ownerSourceMode(owner) != "browser" {
			c.diags.Add("E_BROWSER_CAPABILITY_REQUIRED", diagnostic.SeverityError, "evaluate-js requires browser fetch mode", e.Span, path+".valueSource")
		}
	}
	if field.ValueSource == nil {
		field.ValueSource = ir.TextValueSource{Kind: "text", RawType: rawType, Span: n.Span}
	}
	current := rawType
	for i, a := range applies {
		call, next := c.compileTransformCall(scopeDoc, a, current, fmt.Sprintf("%s.transforms[%d]", path, i))
		field.Transforms = append(field.Transforms, call)
		current = next
	}
	if !typesys.IsAssignable(current, success) {
		c.diags.Add("E_TYPE_MISMATCH", diagnostic.SeverityError, fmt.Sprintf("field pipeline output %s is not assignable to declared type %s", current.String(), success.String()), n.Span, path)
	}
	effective := success
	if !required && field.Default == nil {
		effective = typesys.LiftNullable(success)
	}
	field.EffectiveType = effective
	policy := "null"
	if required {
		policy = "fail"
	}
	if len(onErrors) > 0 {
		oe := onErrors[0]
		validateNode(&c.diags, oe, 1, 1, map[string]valueExpectation{}, path+".onError")
		p, _ := stringArg(oe, 0)
		if p != "fail" && p != "null" && p != "warn" && p != "default" {
			c.diags.Add("E_ERROR_POLICY_INVALID", diagnostic.SeverityError, "on-error must be fail, null, warn, or default", oe.Span, path+".onError")
		} else {
			policy = p
		}
	}
	if (policy == "null" || policy == "warn") && !typesys.IsNullable(effective) {
		c.diags.Add("E_ERROR_POLICY_INVALID", diagnostic.SeverityError, policy+" requires nullable effective type", n.Span, path+".onError")
	}
	if policy == "default" && field.Default == nil {
		c.diags.Add("E_ERROR_POLICY_INVALID", diagnostic.SeverityError, "default policy requires field default", n.Span, path+".onError")
	}
	field.OnError = policy
	return field
}

func (c *Compiler) compileCollection(owner *loadedDocument, n *kdl.Node, path string, scopeDoc *loadedDocument) ir.Collection {
	validateNode(&c.diags, n, 1, 1, map[string]valueExpectation{"required": expectBool, "min-items": expectNonNegativeInt, "max-items": expectNonNegativeInt, "on-row-error": expectString}, path)
	name, ok := stringArg(n, 0)
	if !ok {
		name = "invalid"
		c.diags.Add("E_TYPE_MISMATCH", diagnostic.SeverityError, "collection name must be a string", n.Span, path)
	}
	validateIdentifier(&c.diags, name, n.Span, false, path)
	required, _ := boolProperty(n, "required", false)
	min := 0
	if v, ok := intProperty(n, "min-items"); ok {
		min = v
	}
	if required && min < 1 {
		min = 1
	}
	var max *int
	if v, ok := intProperty(n, "max-items"); ok {
		max = &v
		if v < min {
			c.diags.Add("E_COLLECTION_BOUNDS", diagnostic.SeverityError, "max-items must be >= effective min-items", n.Span, path)
		}
	}
	onRow, _ := stringProperty(n, "on-row-error")
	if onRow == "" {
		onRow = "fail"
	}
	if onRow != "fail" && onRow != "skip" {
		c.diags.Add("E_TYPE_MISMATCH", diagnostic.SeverityError, "on-row-error must be fail or skip", n.Span, path)
	}
	var selects []*kdl.Node
	var members []*kdl.Node
	for _, ch := range n.Children {
		if ch.Name == "select" {
			selects = append(selects, ch)
		} else if ch.Name == "field" || ch.Name == "collection" {
			members = append(members, ch)
		} else {
			c.diags.Add("E_UNKNOWN_NODE", diagnostic.SeverityError, fmt.Sprintf("node %q is not allowed in collection", ch.Name), ch.Span, path)
		}
	}
	selector := ""
	if len(selects) != 1 {
		c.diags.Add("E_SELECTOR_REQUIRED", diagnostic.SeverityError, fmt.Sprintf("collection requires exactly one select; found %d", len(selects)), n.Span, path+".selection")
	} else {
		s := selects[0]
		validateNode(&c.diags, s, 1, 1, map[string]valueExpectation{}, path+".selection")
		selector, _ = stringArg(s, 0)
		c.checkSelector(selector, s, path+".selection")
	}
	if len(members) == 0 {
		c.diags.Add("E_COLLECTION_EMPTY_SCHEMA", diagnostic.SeverityError, "collection requires at least one output member", n.Span, path)
	}
	if ownerSourceMode(owner) == "browser" {
		c.capabilities["browser.query"] = struct{}{}
	}
	rowPath := path + "[]"
	row := c.compileOutputObject(owner, members, rowPath, scopeDoc)
	return ir.Collection{Kind: "collection", ID: path, Name: name, Selector: selector, Required: required, MinItems: min, MaxItems: max, OnRowError: onRow, Row: row, Span: n.Span}
}

func (c *Compiler) addReadCapability(owner *loadedDocument, kind string) {
	if ownerSourceMode(owner) != "browser" {
		return
	}
	c.capabilities["browser.query"] = struct{}{}
	c.capabilities["browser.read-"+kind] = struct{}{}
}
func ownerSourceMode(d *loadedDocument) string {
	if d == nil || d.root == nil {
		return ""
	}
	for _, n := range d.root.Children {
		if n.Name != "source" {
			continue
		}
		for _, ch := range n.Children {
			if ch.Name == "fetch" {
				m, _ := stringProperty(ch, "mode")
				return m
			}
		}
	}
	return ""
}
