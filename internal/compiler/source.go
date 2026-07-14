package compiler

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/hsblabs/scrape-kdl/internal/diagnostic"
	"github.com/hsblabs/scrape-kdl/internal/ir"
	"github.com/hsblabs/scrape-kdl/internal/kdl"
	"github.com/hsblabs/scrape-kdl/internal/limits"
)

func (c *Compiler) compileInputs(d *loadedDocument) ([]ir.Input, map[string]ir.Input) {
	out := []ir.Input{}
	byName := map[string]ir.Input{}
	for _, n := range d.root.Children {
		if n.Name != "input" {
			continue
		}
		validateNode(&c.diags, n, 1, 1, map[string]valueExpectation{"type": expectString, "required": expectBool, "default": expectScalar}, "inputs")
		name, ok := stringArg(n, 0)
		if !ok {
			c.diags.Add("E_TYPE_MISMATCH", diagnostic.SeverityError, "input name must be a string", n.Span, "inputs")
			continue
		}
		validateIdentifier(&c.diags, name, n.Span, false, "inputs."+name)
		if _, exists := byName[name]; exists {
			c.diags.Add("E_DUPLICATE_SYMBOL", diagnostic.SeverityError, fmt.Sprintf("duplicate input %q", name), n.Span, "inputs."+name)
			continue
		}
		typeName, ok := stringProperty(n, "type")
		if !ok || (typeName != "string" && typeName != "bool" && typeName != "int" && typeName != "float") {
			c.diags.Add("E_TYPE_UNKNOWN", diagnostic.SeverityError, "input type must be string, bool, int, or float", n.Span, "inputs."+name)
			continue
		}
		required, ok := boolProperty(n, "required", true)
		if !ok {
			required = true
		}
		item := ir.Input{Name: name, Type: typeName, Required: required, Span: n.Span}
		if def, has := n.Property("default"); has {
			if required {
				c.diags.Add("E_INPUT_REQUIRED_DEFAULT", diagnostic.SeverityError, "required input must not declare default", def.Span, "inputs."+name)
			}
			if !inputDefaultCompatible(def, typeName) {
				c.diags.Add("E_DEFAULT_INVALID", diagnostic.SeverityError, fmt.Sprintf("default is incompatible with input type %s", typeName), def.Span, "inputs."+name)
			}
			raw, _ := rawJSONPtr(def)
			item.Default = raw
		}
		out = append(out, item)
		byName[name] = item
	}
	return out, byName
}

func inputDefaultCompatible(v kdl.Value, typ string) bool {
	switch typ {
	case "string":
		return v.Kind == kdl.ValueString
	case "bool":
		return v.Kind == kdl.ValueBool
	case "int":
		return v.Kind == kdl.ValueInt
	case "float":
		return v.Kind == kdl.ValueInt || v.Kind == kdl.ValueFloat
	default:
		return false
	}
}

func (c *Compiler) compileSource(d *loadedDocument, inputs map[string]ir.Input) ir.Source {
	var nodes []*kdl.Node
	for _, n := range d.root.Children {
		if n.Name == "source" {
			nodes = append(nodes, n)
		}
	}
	if len(nodes) != 1 {
		c.diags.Add("E_DOCUMENT_ROOT", diagnostic.SeverityError, fmt.Sprintf("extractor requires exactly one source; found %d", len(nodes)), d.root.Span, "source")
		return ir.Source{Kind: "html", SessionPolicy: "none", Workflow: []ir.WorkflowStep{}, Span: d.root.Span}
	}
	n := nodes[0]
	validateNode(&c.diags, n, 1, 1, map[string]valueExpectation{}, "source")
	kind, ok := stringArg(n, 0)
	if !ok || kind != "html" {
		c.diags.Add("E_TYPE_MISMATCH", diagnostic.SeverityError, "source argument must be \"html\"", n.Span, "source")
	}
	var fetchNodes, sessionNodes, workflowNodes []*kdl.Node
	for _, ch := range n.Children {
		switch ch.Name {
		case "fetch":
			fetchNodes = append(fetchNodes, ch)
		case "session":
			sessionNodes = append(sessionNodes, ch)
		case "workflow":
			workflowNodes = append(workflowNodes, ch)
		default:
			c.diags.Add("E_UNKNOWN_NODE", diagnostic.SeverityError, fmt.Sprintf("node %q is not allowed in source", ch.Name), ch.Span, "source")
		}
	}
	fetch := ir.Fetch{Mode: "http", URLTemplate: ir.Template{Segments: []ir.TemplateSegment{}}, Span: n.Span}
	if len(fetchNodes) != 1 {
		c.diags.Add("E_DOCUMENT_ROOT", diagnostic.SeverityError, fmt.Sprintf("source requires exactly one fetch; found %d", len(fetchNodes)), n.Span, "source.fetch")
	} else {
		fetch = c.compileFetch(fetchNodes[0], inputs)
	}
	policy := "none"
	if len(sessionNodes) > 1 {
		c.diags.Add("E_DOCUMENT_ROOT", diagnostic.SeverityError, "source allows at most one session", sessionNodes[1].Span, "source.session")
	}
	if len(sessionNodes) > 0 {
		sn := sessionNodes[0]
		validateNode(&c.diags, sn, 0, 0, map[string]valueExpectation{"policy": expectString}, "source.session")
		p, ok := stringProperty(sn, "policy")
		if !ok || (p != "none" && p != "optional" && p != "required") {
			c.diags.Add("E_TYPE_MISMATCH", diagnostic.SeverityError, "session policy must be none, optional, or required", sn.Span, "source.session")
		} else {
			policy = p
		}
	}
	workflow := []ir.WorkflowStep{}
	if len(workflowNodes) > 1 {
		c.diags.Add("E_DOCUMENT_ROOT", diagnostic.SeverityError, "source allows at most one workflow", workflowNodes[1].Span, "source.workflow")
	}
	if len(workflowNodes) > 0 {
		if fetch.Mode != "browser" {
			c.diags.Add("E_BROWSER_CAPABILITY_REQUIRED", diagnostic.SeverityError, "workflow requires browser fetch mode", workflowNodes[0].Span, "source.workflow")
		}
		workflow = c.compileWorkflow(workflowNodes[0])
	}
	if fetch.Mode == "http" {
		c.capabilities["http.fetch"] = struct{}{}
	} else {
		c.capabilities["browser.navigate"] = struct{}{}
	}
	return ir.Source{Kind: "html", Fetch: fetch, SessionPolicy: policy, Workflow: workflow, Span: n.Span}
}

func (c *Compiler) compileFetch(n *kdl.Node, inputs map[string]ir.Input) ir.Fetch {
	validateNode(&c.diags, n, 0, 0, map[string]valueExpectation{"mode": expectString, "url": expectString}, "source.fetch")
	mode, ok := stringProperty(n, "mode")
	if !ok || (mode != "http" && mode != "browser") {
		c.diags.Add("E_TYPE_MISMATCH", diagnostic.SeverityError, "fetch mode must be http or browser", n.Span, "source.fetch")
		mode = "http"
	}
	url, ok := stringProperty(n, "url")
	if !ok || url == "" {
		c.diags.Add("E_TEMPLATE_INVALID", diagnostic.SeverityError, "fetch requires non-empty string url", n.Span, "source.fetch")
		url = ""
	}
	tmpl := c.compileTemplate(url, n, inputs)
	return ir.Fetch{Mode: mode, URLTemplate: tmpl, Span: n.Span}
}

func (c *Compiler) compileTemplate(raw string, n *kdl.Node, inputs map[string]ir.Input) ir.Template {
	segments := []ir.TemplateSegment{}
	var lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			segments = append(segments, ir.LiteralTemplateSegment{Kind: "literal", Value: lit.String()})
			lit.Reset()
		}
	}
	for i := 0; i < len(raw); {
		switch raw[i] {
		case '{':
			if i+1 < len(raw) && raw[i+1] == '{' {
				lit.WriteByte('{')
				i += 2
				continue
			}
			flush()
			end := strings.IndexByte(raw[i+1:], '}')
			if end < 0 {
				c.diags.Add("E_TEMPLATE_INVALID", diagnostic.SeverityError, "unmatched { in URL template", n.Span, "source.fetch.url")
				lit.WriteString(raw[i:])
				i = len(raw)
				continue
			}
			end = i + 1 + end
			name := raw[i+1 : end]
			if !userNamePattern.MatchString(name) {
				c.diags.Add("E_TEMPLATE_INVALID", diagnostic.SeverityError, fmt.Sprintf("invalid placeholder %q", name), n.Span, "source.fetch.url")
			} else if inp, ok := inputs[name]; !ok {
				c.diags.Add("E_INPUT_UNDECLARED", diagnostic.SeverityError, fmt.Sprintf("URL template references undeclared input %q", name), n.Span, "source.fetch.url")
			} else if !inp.Required && inp.Default == nil {
				c.diags.Add("E_TEMPLATE_OPTIONAL_INPUT", diagnostic.SeverityError, fmt.Sprintf("optional input %q used by URL template requires default", name), n.Span, "source.fetch.url")
			}
			segments = append(segments, ir.InputTemplateSegment{Kind: "input", Name: name})
			i = end + 1
		case '}':
			if i+1 < len(raw) && raw[i+1] == '}' {
				lit.WriteByte('}')
				i += 2
				continue
			}
			c.diags.Add("E_TEMPLATE_INVALID", diagnostic.SeverityError, "unmatched } in URL template", n.Span, "source.fetch.url")
			i++
		default:
			lit.WriteByte(raw[i])
			i++
		}
	}
	flush()
	var probe strings.Builder
	for _, segment := range segments {
		switch value := segment.(type) {
		case ir.LiteralTemplateSegment:
			probe.WriteString(value.Value)
		case ir.InputTemplateSegment:
			probe.WriteString("x")
		}
	}
	parsed, err := url.Parse(probe.String())
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		c.diags.Add("E_TEMPLATE_INVALID", diagnostic.SeverityError, "expanded URL template must be an absolute http or https URL", n.Span, "source.fetch.url")
	}
	return ir.Template{Raw: raw, Segments: segments}
}

func (c *Compiler) compileWorkflow(n *kdl.Node) []ir.WorkflowStep {
	validateNode(&c.diags, n, 0, 0, map[string]valueExpectation{}, "source.workflow")
	out := []ir.WorkflowStep{}
	for i, ch := range n.Children {
		path := fmt.Sprintf("source.workflow[%d]", i)
		switch ch.Name {
		case "wait-for":
			validateNode(&c.diags, ch, 1, 1, map[string]valueExpectation{"state": expectString, "timeout-ms": expectNonNegativeInt}, path)
			sel, _ := stringArg(ch, 0)
			c.checkSelector(sel, ch, path)
			state, _ := stringProperty(ch, "state")
			if state == "" {
				state = "visible"
			}
			if state != "attached" && state != "visible" && state != "hidden" && state != "detached" {
				c.diags.Add("E_TYPE_MISMATCH", diagnostic.SeverityError, "invalid wait-for state", ch.Span, path)
			}
			out = append(out, ir.WaitForStep{Kind: "wait-for", Selector: sel, State: state, TimeoutMS: durationIntPtr(c, ch, "timeout-ms", path), Span: ch.Span})
			c.capabilities["browser.wait"] = struct{}{}
		case "click":
			validateNode(&c.diags, ch, 1, 1, map[string]valueExpectation{"timeout-ms": expectNonNegativeInt}, path)
			sel, _ := stringArg(ch, 0)
			c.checkSelector(sel, ch, path)
			out = append(out, ir.ClickStep{Kind: "click", Selector: sel, TimeoutMS: durationIntPtr(c, ch, "timeout-ms", path), Span: ch.Span})
			c.capabilities["browser.input"] = struct{}{}
		case "fill":
			validateNode(&c.diags, ch, 2, 2, map[string]valueExpectation{"timeout-ms": expectNonNegativeInt}, path)
			sel, _ := stringArg(ch, 0)
			val, _ := stringArg(ch, 1)
			c.checkSelector(sel, ch, path)
			out = append(out, ir.FillStep{Kind: "fill", Selector: sel, Value: val, TimeoutMS: durationIntPtr(c, ch, "timeout-ms", path), Span: ch.Span})
			c.capabilities["browser.input"] = struct{}{}
		case "press":
			validateNode(&c.diags, ch, 2, 2, map[string]valueExpectation{"timeout-ms": expectNonNegativeInt}, path)
			sel, _ := stringArg(ch, 0)
			key, _ := stringArg(ch, 1)
			c.checkSelector(sel, ch, path)
			out = append(out, ir.PressStep{Kind: "press", Selector: sel, Key: key, TimeoutMS: durationIntPtr(c, ch, "timeout-ms", path), Span: ch.Span})
			c.capabilities["browser.input"] = struct{}{}
		case "scroll":
			validateNode(&c.diags, ch, 2, 2, map[string]valueExpectation{}, path)
			x, xok := floatArg(ch, 0)
			y, yok := floatArg(ch, 1)
			if !xok || !yok {
				c.diags.Add("E_TYPE_MISMATCH", diagnostic.SeverityError, "scroll requires numeric x and y", ch.Span, path)
			}
			out = append(out, ir.ScrollStep{Kind: "scroll", X: x, Y: y, Span: ch.Span})
			c.capabilities["browser.scroll"] = struct{}{}
		case "wait-for-network-idle":
			validateNode(&c.diags, ch, 0, 0, map[string]valueExpectation{"idle-ms": expectNonNegativeInt, "timeout-ms": expectNonNegativeInt}, path)
			idle := 500
			if v, ok := intProperty(ch, "idle-ms"); ok {
				idle = v
				if idle < 1 {
					c.diags.Add("E_TYPE_MISMATCH", diagnostic.SeverityError, "idle-ms must be positive", ch.Span, path)
				} else if int64(idle) > limits.MaxMilliseconds {
					c.diags.Add("E_TYPE_MISMATCH", diagnostic.SeverityError, fmt.Sprintf("idle-ms must not exceed %d", limits.MaxMilliseconds), ch.Span, path)
				}
			}
			out = append(out, ir.NetworkIdleStep{Kind: "wait-for-network-idle", IdleMS: idle, TimeoutMS: durationIntPtr(c, ch, "timeout-ms", path), Span: ch.Span})
			c.capabilities["browser.network-idle"] = struct{}{}
		case "evaluate-js":
			validateNode(&c.diags, ch, 1, 1, map[string]valueExpectation{"timeout-ms": expectNonNegativeInt}, path)
			src, _ := stringArg(ch, 0)
			out = append(out, ir.EvaluateJavaScriptStep{Kind: "evaluate-js", Source: src, TimeoutMS: durationIntPtr(c, ch, "timeout-ms", path), Span: ch.Span})
			c.capabilities["browser.evaluate-js"] = struct{}{}
			c.jsPresent = true
		default:
			c.diags.Add("E_UNKNOWN_NODE", diagnostic.SeverityError, fmt.Sprintf("unknown workflow step %q", ch.Name), ch.Span, path)
		}
	}
	return out
}

func positiveIntPtr(c *Compiler, n *kdl.Node, name, path string) *int {
	v, ok := intProperty(n, name)
	if !ok {
		return nil
	}
	if v < 1 {
		c.diags.Add("E_TYPE_MISMATCH", diagnostic.SeverityError, name+" must be positive", n.Span, path)
		return nil
	}
	return &v
}

func durationIntPtr(c *Compiler, n *kdl.Node, name, path string) *int {
	v, ok := intProperty(n, name)
	if !ok {
		return nil
	}
	if v < 1 {
		c.diags.Add("E_TYPE_MISMATCH", diagnostic.SeverityError, name+" must be positive", n.Span, path)
		return nil
	}
	if int64(v) > limits.MaxMilliseconds {
		c.diags.Add("E_TYPE_MISMATCH", diagnostic.SeverityError, fmt.Sprintf("%s must not exceed %d", name, limits.MaxMilliseconds), n.Span, path)
		return nil
	}
	return &v
}
func (c *Compiler) checkSelector(sel string, n *kdl.Node, path string) {
	if err := validateSelector(sel); err != nil {
		code := "E_SELECTOR_INVALID"
		if strings.Contains(err.Error(), "unsupported") {
			code = "E_SELECTOR_UNSUPPORTED"
		}
		c.diags.Add(code, diagnostic.SeverityError, err.Error(), n.Span, path)
	}
}
