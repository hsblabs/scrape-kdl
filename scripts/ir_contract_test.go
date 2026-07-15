package scripts

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	internalir "github.com/hsblabs/scrape-kdl/internal/ir"
	"github.com/hsblabs/scrape-kdl/internal/source"
)

type declarationShape struct {
	properties []string
	required   []string
}

type declarationMapping struct {
	name   string
	schema string
}

func TestIRDeclarationShapesMatchSchema(t *testing.T) {
	root := repositoryRoot(t)
	schema := loadSchemaShapes(t, filepath.Join(root, "docs", "ir", "2026-07-15", "schema.json"))

	internalMappings := []struct {
		value  any
		schema string
	}{
		{source.Position{}, "sourcePosition"}, {source.Span{}, "sourceSpan"},
		{internalir.SourceFile{}, "sourceFile"}, {internalir.Extractor{}, "extractor"},
		{internalir.Input{}, "input"}, {internalir.Source{}, "source"}, {internalir.Fetch{}, "fetch"}, {internalir.Template{}, "template"},
		{internalir.LiteralTemplateSegment{}, "templateSegment:literal"}, {internalir.InputTemplateSegment{}, "templateSegment:input"},
		{internalir.WaitForStep{}, "workflowStep:wait-for"}, {internalir.ClickStep{}, "workflowStep:click"},
		{internalir.FillStep{}, "workflowStep:fill"}, {internalir.PressStep{}, "workflowStep:press"},
		{internalir.ScrollStep{}, "workflowStep:scroll"}, {internalir.NetworkIdleStep{}, "workflowStep:wait-for-network-idle"},
		{internalir.EvaluateJavaScriptStep{}, "workflowStep:evaluate-js"},
		{internalir.OutputObject{}, "outputObject"}, {internalir.Field{}, "field"}, {internalir.Collection{}, "collection"}, {internalir.FieldSelection{}, "fieldSelection"},
		{internalir.TextValueSource{}, "valueSource:text"}, {internalir.HTMLValueSource{}, "valueSource:html"},
		{internalir.AttributeValueSource{}, "valueSource:attribute"}, {internalir.JavaScriptValueSource{}, "valueSource:javascript"},
		{internalir.PipelineTransform{}, "transform:pipeline"}, {internalir.MatchTransform{}, "transform:match"}, {internalir.ExternalTransform{}, "transform:external"},
		{internalir.MatchCase{}, "matchCase"}, {internalir.TransformCall{}, "transformCall"},
		{internalir.BuiltinTarget{}, "transformTarget:builtin"}, {internalir.DeclaredTarget{}, "transformTarget:declared"}, {internalir.NamedArgument{}, "namedArgument"},
	}
	for _, mapping := range internalMappings {
		name := reflect.TypeOf(mapping.value).Name()
		t.Run("internal/"+name, func(t *testing.T) {
			compareDeclarationShape(t, goReflectionShape(reflect.TypeOf(mapping.value)), schema[mapping.schema])
		})
	}

	goMappings := []declarationMapping{
		{"SourcePosition", "sourcePosition"}, {"SourceSpan", "sourceSpan"}, {"SourceFile", "sourceFile"}, {"Extractor", "extractor"},
		{"Input", "input"}, {"Source", "source"}, {"Fetch", "fetch"}, {"Template", "template"},
		{"LiteralTemplateSegment", "templateSegment:literal"}, {"InputTemplateSegment", "templateSegment:input"},
		{"WaitForStep", "workflowStep:wait-for"}, {"ClickStep", "workflowStep:click"}, {"FillStep", "workflowStep:fill"}, {"PressStep", "workflowStep:press"},
		{"ScrollStep", "workflowStep:scroll"}, {"NetworkIdleStep", "workflowStep:wait-for-network-idle"}, {"EvaluateJavaScriptStep", "workflowStep:evaluate-js"},
		{"OutputObject", "outputObject"}, {"Field", "field"}, {"Collection", "collection"}, {"FieldSelection", "fieldSelection"},
		{"TextValueSource", "valueSource:text"}, {"HTMLValueSource", "valueSource:html"}, {"AttributeValueSource", "valueSource:attribute"}, {"JavaScriptValueSource", "valueSource:javascript"},
		{"PipelineTransform", "transform:pipeline"}, {"MatchTransform", "transform:match"}, {"ExternalTransform", "transform:external"},
		{"MatchCase", "matchCase"}, {"TransformCall", "transformCall"}, {"BuiltinTransformTarget", "transformTarget:builtin"},
		{"DeclaredTransformTarget", "transformTarget:declared"}, {"NamedArgument", "namedArgument"},
	}
	goShapes := parseGoDeclarationShapes(t, filepath.Join(root, "docs", "ir", "go", "ir", "types.go"))
	for _, mapping := range goMappings {
		t.Run("go/"+mapping.name, func(t *testing.T) {
			compareDeclarationShape(t, resolveGoShape(t, mapping.name, goShapes, map[string]bool{}), schema[mapping.schema])
		})
	}

	tsMappings := []declarationMapping{
		{"SourcePosition", "sourcePosition"}, {"SourceSpan", "sourceSpan"}, {"SourceFileIR", "sourceFile"}, {"ExtractorIR", "extractor"},
		{"InputIR", "input"}, {"SourceIR", "source"}, {"FetchIR", "fetch"}, {"TemplateIR", "template"},
		{"LiteralTemplateSegmentIR", "templateSegment:literal"}, {"InputTemplateSegmentIR", "templateSegment:input"},
		{"WaitForStepIR", "workflowStep:wait-for"}, {"ClickStepIR", "workflowStep:click"}, {"FillStepIR", "workflowStep:fill"}, {"PressStepIR", "workflowStep:press"},
		{"ScrollStepIR", "workflowStep:scroll"}, {"NetworkIdleStepIR", "workflowStep:wait-for-network-idle"}, {"EvaluateJavaScriptStepIR", "workflowStep:evaluate-js"},
		{"OutputObjectIR", "outputObject"}, {"FieldIR", "field"}, {"CollectionIR", "collection"}, {"FieldSelectionIR", "fieldSelection"},
		{"TextValueSourceIR", "valueSource:text"}, {"HtmlValueSourceIR", "valueSource:html"}, {"AttributeValueSourceIR", "valueSource:attribute"}, {"JavaScriptValueSourceIR", "valueSource:javascript"},
		{"PipelineTransformIR", "transform:pipeline"}, {"MatchTransformIR", "transform:match"}, {"ExternalTransformIR", "transform:external"},
		{"MatchCaseIR", "matchCase"}, {"ResolvedTransformCallIR", "transformCall"}, {"BuiltinTransformTargetIR", "transformTarget:builtin"},
		{"DeclaredTransformTargetIR", "transformTarget:declared"}, {"NamedArgumentIR", "namedArgument"},
	}
	typeScriptDeclarations := []struct {
		name string
		path string
	}{
		{"typescript-docs", filepath.Join(root, "docs", "ir", "typescript", "index.ts")},
		{"typescript-package", filepath.Join(root, "packages", "scrape-kdl", "src", "ir.ts")},
	}
	for _, declarations := range typeScriptDeclarations {
		tsShapes := parseTypeScriptDeclarationShapes(t, declarations.path)
		for _, mapping := range tsMappings {
			t.Run(declarations.name+"/"+mapping.name, func(t *testing.T) {
				compareDeclarationShape(t, resolveTypeScriptShape(t, mapping.name, tsShapes, map[string]bool{}), schema[mapping.schema])
			})
		}
	}
}

func loadSchemaShapes(t *testing.T, path string) map[string]declarationShape {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	definitions := document["$defs"].(map[string]any)
	shapes := map[string]declarationShape{}
	for name, raw := range definitions {
		definition := raw.(map[string]any)
		if _, ok := definition["properties"]; ok {
			shapes[name] = schemaObjectShape(definition)
		}
		for _, rawVariant := range anySlice(definition["oneOf"]) {
			variant := rawVariant.(map[string]any)
			properties, ok := variant["properties"].(map[string]any)
			if !ok {
				continue
			}
			kind, ok := properties["kind"].(map[string]any)["const"].(string)
			if ok {
				shapes[name+":"+kind] = schemaObjectShape(variant)
			}
		}
	}
	return shapes
}

func schemaObjectShape(object map[string]any) declarationShape {
	properties := object["properties"].(map[string]any)
	propertyNames := make([]string, 0, len(properties))
	for name := range properties {
		propertyNames = append(propertyNames, name)
	}
	required := stringSlice(object["required"])
	slices.Sort(propertyNames)
	slices.Sort(required)
	return declarationShape{properties: propertyNames, required: required}
}

func goReflectionShape(typ reflect.Type) declarationShape {
	properties := map[string]bool{}
	var collect func(reflect.Type)
	collect = func(current reflect.Type) {
		for index := 0; index < current.NumField(); index++ {
			field := current.Field(index)
			tag := field.Tag.Get("json")
			if field.Anonymous && tag == "" {
				collect(field.Type)
				continue
			}
			parts := strings.Split(tag, ",")
			if len(parts) == 0 || parts[0] == "" || parts[0] == "-" {
				continue
			}
			properties[parts[0]] = !slices.Contains(parts[1:], "omitempty")
		}
	}
	collect(typ)
	return mapShape(properties)
}

type goDeclaration struct {
	properties map[string]bool
	embedded   []string
}

func parseGoDeclarationShapes(t *testing.T, path string) map[string]goDeclaration {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	declarations := map[string]goDeclaration{}
	for _, declaration := range file.Decls {
		generic, ok := declaration.(*ast.GenDecl)
		if !ok || generic.Tok != token.TYPE {
			continue
		}
		for _, spec := range generic.Specs {
			typeSpec := spec.(*ast.TypeSpec)
			structure, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}
			shape := goDeclaration{properties: map[string]bool{}}
			for _, field := range structure.Fields.List {
				if len(field.Names) == 0 {
					if identifier, ok := field.Type.(*ast.Ident); ok {
						shape.embedded = append(shape.embedded, identifier.Name)
					}
					continue
				}
				if field.Tag == nil {
					continue
				}
				rawTag, err := strconv.Unquote(field.Tag.Value)
				if err != nil {
					t.Fatal(err)
				}
				parts := strings.Split(reflect.StructTag(rawTag).Get("json"), ",")
				if parts[0] != "" && parts[0] != "-" {
					shape.properties[parts[0]] = !slices.Contains(parts[1:], "omitempty")
				}
			}
			declarations[typeSpec.Name.Name] = shape
		}
	}
	return declarations
}

func resolveGoShape(t *testing.T, name string, declarations map[string]goDeclaration, visiting map[string]bool) declarationShape {
	t.Helper()
	if visiting[name] {
		t.Fatalf("embedded Go declaration cycle at %s", name)
	}
	declaration, ok := declarations[name]
	if !ok {
		t.Fatalf("missing Go declaration %s", name)
	}
	visiting[name] = true
	properties := map[string]bool{}
	for property, required := range declaration.properties {
		properties[property] = required
	}
	for _, embedded := range declaration.embedded {
		mergeShape(properties, resolveGoShape(t, embedded, declarations, visiting))
	}
	delete(visiting, name)
	return mapShape(properties)
}

type typeScriptDeclaration struct {
	properties map[string]bool
	extends    string
}

func parseTypeScriptDeclarationShapes(t *testing.T, path string) map[string]typeScriptDeclaration {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	interfacePattern := regexp.MustCompile(`(?ms)^(?:export )?interface ([A-Za-z0-9_]+)(?: extends ([A-Za-z0-9_]+))? \{\n(.*?)^\}`)
	propertyPattern := regexp.MustCompile(`(?m)^\s+readonly ([A-Za-z0-9_]+)(\?)?:`)
	declarations := map[string]typeScriptDeclaration{}
	for _, match := range interfacePattern.FindAllStringSubmatch(string(data), -1) {
		declaration := typeScriptDeclaration{properties: map[string]bool{}, extends: match[2]}
		for _, property := range propertyPattern.FindAllStringSubmatch(match[3], -1) {
			declaration.properties[property[1]] = property[2] != "?"
		}
		declarations[match[1]] = declaration
	}
	return declarations
}

func resolveTypeScriptShape(t *testing.T, name string, declarations map[string]typeScriptDeclaration, visiting map[string]bool) declarationShape {
	t.Helper()
	if visiting[name] {
		t.Fatalf("TypeScript declaration inheritance cycle at %s", name)
	}
	declaration, ok := declarations[name]
	if !ok {
		t.Fatalf("missing TypeScript declaration %s", name)
	}
	visiting[name] = true
	properties := map[string]bool{}
	for property, required := range declaration.properties {
		properties[property] = required
	}
	if declaration.extends != "" {
		mergeShape(properties, resolveTypeScriptShape(t, declaration.extends, declarations, visiting))
	}
	delete(visiting, name)
	return mapShape(properties)
}

func compareDeclarationShape(t *testing.T, got, want declarationShape) {
	t.Helper()
	if !slices.Equal(got.properties, want.properties) || !slices.Equal(got.required, want.required) {
		t.Fatalf("declaration shape = properties %v required %v; schema = properties %v required %v", got.properties, got.required, want.properties, want.required)
	}
}

func mergeShape(target map[string]bool, shape declarationShape) {
	for _, property := range shape.properties {
		target[property] = slices.Contains(shape.required, property)
	}
}

func mapShape(properties map[string]bool) declarationShape {
	shape := declarationShape{}
	for property, required := range properties {
		shape.properties = append(shape.properties, property)
		if required {
			shape.required = append(shape.required, property)
		}
	}
	slices.Sort(shape.properties)
	slices.Sort(shape.required)
	return shape
}

func anySlice(value any) []any {
	if value == nil {
		return nil
	}
	return value.([]any)
}

func stringSlice(value any) []string {
	items := anySlice(value)
	strings := make([]string, len(items))
	for index, item := range items {
		strings[index] = item.(string)
	}
	return strings
}
