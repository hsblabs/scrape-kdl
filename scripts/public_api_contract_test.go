package scripts

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestGoPublicSignaturesDoNotReferenceInternalPackages(t *testing.T) {
	root := repositoryRoot(t)
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, "scrapekdl.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	imports := map[string]string{}
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(path)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		imports[name] = path
	}

	check := func(owner string, node ast.Node) {
		ast.Inspect(node, func(current ast.Node) bool {
			selector, ok := current.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			identifier, ok := selector.X.(*ast.Ident)
			if ok && strings.Contains(imports[identifier.Name], "/internal/") {
				t.Errorf("public signature %s references internal package %s", owner, imports[identifier.Name])
			}
			return true
		})
	}

	for _, declaration := range file.Decls {
		switch typed := declaration.(type) {
		case *ast.FuncDecl:
			publicReceiver := typed.Recv == nil
			if typed.Recv != nil && len(typed.Recv.List) == 1 {
				receiver := typed.Recv.List[0].Type
				if pointer, ok := receiver.(*ast.StarExpr); ok {
					receiver = pointer.X
				}
				if identifier, ok := receiver.(*ast.Ident); ok {
					publicReceiver = identifier.IsExported()
				}
			}
			if publicReceiver && typed.Name.IsExported() {
				check(typed.Name.Name, typed.Type)
			}
		case *ast.GenDecl:
			for _, rawSpec := range typed.Specs {
				spec, ok := rawSpec.(*ast.TypeSpec)
				if !ok || !spec.Name.IsExported() {
					continue
				}
				if structure, ok := spec.Type.(*ast.StructType); ok {
					for _, field := range structure.Fields.List {
						if len(field.Names) == 0 || field.Names[0].IsExported() {
							check(spec.Name.Name, field.Type)
						}
					}
					continue
				}
				check(spec.Name.Name, spec.Type)
			}
		}
	}
}
