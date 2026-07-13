package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hsblabs/scrape-kdl/internal/diagnostic"
	"github.com/hsblabs/scrape-kdl/internal/ir"
	"github.com/hsblabs/scrape-kdl/internal/kdl"
	"github.com/hsblabs/scrape-kdl/internal/source"
	"github.com/hsblabs/scrape-kdl/internal/typesys"
)

type documentKind string

const (
	docExtractor documentKind = "extractor"
	docModule    documentKind = "module"
)

type loadedDocument struct {
	path           string
	displayPath    string
	data           []byte
	doc            *kdl.Document
	kind           documentKind
	root           *kdl.Node
	imports        map[string]*loadedDocument
	importOrder    []string
	transformDecls map[string]*transformDecl
	moduleName     string
	moduleVersion  int
}

type transformDecl struct {
	doc       *loadedDocument
	node      *kdl.Node
	name      string
	symbolID  string
	input     typesys.Type
	output    typesys.Type
	compiled  ir.Transform
	compiling bool
}

type Compiler struct {
	diags        diagnostic.List
	documents    map[string]*loadedDocument
	loading      map[string]bool
	files        []ir.SourceFile
	capabilities map[string]struct{}
	jsPresent    bool
	entryDir     string
}

func New() *Compiler {
	return &Compiler{
		documents:    map[string]*loadedDocument{},
		loading:      map[string]bool{},
		capabilities: map[string]struct{}{},
	}
}

func CompileFile(path string) (*ir.Extractor, diagnostic.List) {
	c := New()
	absPath := absClean(path)
	c.entryDir = filepath.Dir(absPath)
	root := c.loadDocument(absPath, docExtractor)
	if root == nil || c.diags.HasErrors() {
		return nil, c.diags.Sorted()
	}
	out := c.compileExtractor(root)
	if c.jsPresent {
		c.diags.Add("W_JAVASCRIPT_PRESENT", diagnostic.SeverityWarning, "specification contains trusted JavaScript execution", root.root.Span, "source")
	}
	if c.diags.HasErrors() {
		return nil, c.diags.Sorted()
	}
	return out, c.diags.Sorted()
}

func ValidateFile(path string) diagnostic.List {
	_, diags := CompileFile(path)
	return diags
}

func (c *Compiler) loadDocument(path string, expected documentKind) *loadedDocument {
	path = absClean(path)
	if c.loading[path] {
		span := zeroSpan(c.displayPath(path))
		c.diags.Add("E_IMPORT_CYCLE", diagnostic.SeverityError, fmt.Sprintf("import cycle includes %q", c.displayPath(path)), span, "")
		return nil
	}
	if d, ok := c.documents[path]; ok {
		if expected == docModule && d.kind != docModule {
			c.diags.Add("E_IMPORT_KIND", diagnostic.SeverityError, fmt.Sprintf("imported file %q is not a module document", path), d.doc.Span, "")
		}
		return d
	}
	c.loading[path] = true
	defer delete(c.loading, path)

	data, err := os.ReadFile(path)
	if err != nil {
		c.diags.Add("E_KDL_SYNTAX", diagnostic.SeverityError, fmt.Sprintf("read %q: %v", c.displayPath(path), err), zeroSpan(c.displayPath(path)), "")
		return nil
	}
	displayPath := c.displayPath(path)
	doc, parseDiags := kdl.Parse(displayPath, data)
	c.diags = append(c.diags, parseDiags...)
	if parseDiags.HasErrors() {
		return nil
	}

	d := &loadedDocument{path: path, displayPath: displayPath, data: data, doc: doc, imports: map[string]*loadedDocument{}, transformDecls: map[string]*transformDecl{}}
	c.documents[path] = d
	c.files = append(c.files, ir.SourceFile{Path: displayPath, SHA256: sha256Hex(data)})

	var roots []*kdl.Node
	seenRoot := false
	for _, n := range doc.Nodes {
		if n.Name == "import" {
			if seenRoot {
				c.diags.Add("E_DOCUMENT_ROOT", diagnostic.SeverityError, "imports must appear before the document root", n.Span, "")
			}
			c.compileImportHeader(d, n)
			continue
		}
		seenRoot = true
		if n.Name == "extractor" || n.Name == "module" {
			roots = append(roots, n)
		} else {
			c.diags.Add("E_UNKNOWN_NODE", diagnostic.SeverityError, fmt.Sprintf("top-level node %q is not allowed", n.Name), n.Span, "")
		}
	}
	if len(roots) != 1 {
		c.diags.Add("E_DOCUMENT_ROOT", diagnostic.SeverityError, fmt.Sprintf("document must contain exactly one extractor or module root; found %d", len(roots)), doc.Span, "")
		return d
	}
	d.root = roots[0]
	if d.root.Name == "extractor" {
		d.kind = docExtractor
	} else {
		d.kind = docModule
	}
	if expected == docModule && d.kind != docModule {
		c.diags.Add("E_IMPORT_KIND", diagnostic.SeverityError, "import target must be a module document", d.root.Span, "")
	}
	if expected == docExtractor && d.kind != docExtractor {
		c.diags.Add("E_DOCUMENT_ROOT", diagnostic.SeverityError, "entry document must be an extractor document", d.root.Span, "")
	}

	c.compileRootHeader(d)
	for _, alias := range d.importOrder {
		imp := d.imports[alias]
		if imp != nil {
			c.collectTransformDecls(imp)
		}
	}
	c.collectTransformDecls(d)
	return d
}

func (c *Compiler) compileImportHeader(d *loadedDocument, n *kdl.Node) {
	validateNode(&c.diags, n, 1, 1, map[string]valueExpectation{"as": expectString}, "imports")
	pathArg, ok := stringArg(n, 0)
	if !ok {
		c.diags.Add("E_TYPE_MISMATCH", diagnostic.SeverityError, "import path must be a string", n.Span, "imports")
		return
	}
	alias, ok := stringProperty(n, "as")
	if !ok || alias == "" {
		c.diags.Add("E_IMPORT_ALIAS_REQUIRED", diagnostic.SeverityError, "import requires non-empty string property as", n.Span, "imports")
		return
	}
	validateIdentifier(&c.diags, alias, n.Span, false, "imports")
	if _, exists := d.imports[alias]; exists {
		c.diags.Add("E_DUPLICATE_SYMBOL", diagnostic.SeverityError, fmt.Sprintf("duplicate import alias %q", alias), n.Span, "imports")
		return
	}
	if strings.Contains(pathArg, "://") || filepath.IsAbs(pathArg) {
		c.diags.Add("E_REMOTE_IMPORT_UNSUPPORTED", diagnostic.SeverityError, "import path must be relative", n.Span, "imports")
		return
	}
	resolved := absClean(filepath.Join(filepath.Dir(d.path), filepath.FromSlash(pathArg)))
	imp := c.loadDocument(resolved, docModule)
	d.imports[alias] = imp
	d.importOrder = append(d.importOrder, alias)
}

func (c *Compiler) compileRootHeader(d *loadedDocument) {
	validateNode(&c.diags, d.root, 1, 1, map[string]valueExpectation{"version": expectInt}, d.root.Name)
	name, ok := stringArg(d.root, 0)
	if !ok {
		c.diags.Add("E_TYPE_MISMATCH", diagnostic.SeverityError, d.root.Name+" name must be a string", d.root.Span, d.root.Name)
		return
	}
	validateIdentifier(&c.diags, name, d.root.Span, true, d.root.Name)
	version, ok := intProperty(d.root, "version")
	if !ok || version < 1 {
		c.diags.Add("E_TYPE_MISMATCH", diagnostic.SeverityError, "version must be a positive integer", d.root.Span, d.root.Name)
	}
	if d.kind == docModule {
		for _, child := range d.root.Children {
			if child.Name != "transform" {
				c.diags.Add("E_UNKNOWN_NODE", diagnostic.SeverityError, fmt.Sprintf("node %q is not allowed in module", child.Name), child.Span, "module")
			}
		}
		d.moduleName = name
		d.moduleVersion = version
		for i := range c.files {
			if c.files[i].Path == d.displayPath {
				c.files[i].ModuleName = name
				c.files[i].ModuleVersion = version
			}
		}
	}
}

func (c *Compiler) collectTransformDecls(d *loadedDocument) {
	if d == nil || d.root == nil || len(d.transformDecls) > 0 {
		return
	}
	for _, child := range d.root.Children {
		if child.Name != "transform" {
			continue
		}
		validateNode(&c.diags, child, 1, 1, map[string]valueExpectation{"input": expectString, "output": expectString}, "transforms")
		name, ok := stringArg(child, 0)
		if !ok {
			c.diags.Add("E_TYPE_MISMATCH", diagnostic.SeverityError, "transform name must be a string", child.Span, "transforms")
			continue
		}
		validateIdentifier(&c.diags, name, child.Span, false, "transforms."+name)
		if isBuiltin(name) {
			c.diags.Add("E_TRANSFORM_SHADOWS_BUILTIN", diagnostic.SeverityError, fmt.Sprintf("transform %q shadows built-in", name), child.Span, "transforms."+name)
		}
		if _, exists := d.transformDecls[name]; exists {
			c.diags.Add("E_DUPLICATE_SYMBOL", diagnostic.SeverityError, fmt.Sprintf("duplicate transform %q", name), child.Span, "transforms."+name)
			continue
		}
		inS, inOK := stringProperty(child, "input")
		outS, outOK := stringProperty(child, "output")
		if !inOK || !outOK {
			c.diags.Add("E_TYPE_UNKNOWN", diagnostic.SeverityError, "transform requires string input and output properties", child.Span, "transforms."+name)
			continue
		}
		inT, err := typesys.Parse(inS)
		if err != nil {
			c.diags.Add("E_TYPE_UNKNOWN", diagnostic.SeverityError, err.Error(), child.Span, "transforms."+name+".input")
			continue
		}
		outT, err := typesys.Parse(outS)
		if err != nil {
			c.diags.Add("E_TYPE_UNKNOWN", diagnostic.SeverityError, err.Error(), child.Span, "transforms."+name+".output")
			continue
		}
		d.transformDecls[name] = &transformDecl{doc: d, node: child, name: name, symbolID: d.displayPath + "#transform:" + name, input: inT, output: outT}
	}
}

func (c *Compiler) compileExtractor(d *loadedDocument) *ir.Extractor {
	if d == nil || d.root == nil {
		return nil
	}
	name, _ := stringArg(d.root, 0)
	version, _ := intProperty(d.root, "version")
	inputs, inputMap := c.compileInputs(d)
	sourceIR := c.compileSource(d, inputMap)
	transforms := c.compileReachableTransforms(d)
	output := c.compileOutputObject(d, d.root.Children, "output", d)

	files := append([]ir.SourceFile{}, c.files...)
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return &ir.Extractor{Kind: "extractor", IRVersion: "0.1", LanguageVersion: "0.1", Name: name, Version: version, Files: files, Source: sourceIR, Inputs: inputs, Transforms: transforms, Output: output, Capabilities: sortedSet(c.capabilities), Span: d.root.Span}
}

func (c *Compiler) displayPath(path string) string {
	if c.entryDir == "" {
		return filepath.ToSlash(filepath.Clean(path))
	}
	rel, err := filepath.Rel(c.entryDir, path)
	if err != nil {
		return filepath.ToSlash(filepath.Clean(path))
	}
	return filepath.ToSlash(rel)
}

func zeroSpan(path string) source.Span {
	p := source.Position{Offset: 0, Line: 1, Column: 1}
	return source.Span{File: path, Start: p, End: p}
}
