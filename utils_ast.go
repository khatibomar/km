package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
)

func loadAstFromFile(path string) (*ast.File, error) {
	fset := token.NewFileSet()
	return parser.ParseFile(fset, path, nil, 0)
}

func getPackage(node *ast.File) string {
	return node.Name.String()
}

func getImports(node *ast.File) []string {
	var res []string
	for _, i := range node.Imports {
		res = append(res, i.Path.Value[1:len(i.Path.Value)-1])
	}

	return res
}

func getFields(node *ast.File, typeName string) ([]Field, error) {
	var fields []Field
	var found bool
	var err error

	ast.Inspect(node, func(n ast.Node) bool {
		if typeSpec, ok := n.(*ast.TypeSpec); ok {
			if typeSpec.Name.String() == typeName {
				found = true
				if structType, ok := typeSpec.Type.(*ast.StructType); ok {
					for _, f := range structType.Fields.List {
						for _, n := range f.Names {
							fields = append(fields, Field{
								Name: n.Name,
								Type: extractTypeFromExpression(f.Type),
							})
						}
					}
				}
				return false
			}
		}
		return true
	})

	if !found {
		err = fmt.Errorf("type(%s): %w", typeName, errTypeNotFound)
	}

	return fields, err
}

func extractTypeFromExpression(expr ast.Expr) string {
	switch expr := expr.(type) {
	case *ast.Ident:
		return expr.Name
	case *ast.StarExpr:
		return "*" + extractTypeFromExpression(expr.X)
	case *ast.ArrayType:
		return "[]" + extractTypeFromExpression(expr.Elt)
	case *ast.MapType:
		return "map[" + extractTypeFromExpression(expr.Key) + "]" + extractTypeFromExpression(expr.Value)
	case *ast.StructType:
		return "struct{}"
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.ChanType:
		var dir string
		switch expr.Dir {
		case ast.SEND:
			dir = "chan<- "
		case ast.RECV:
			dir = "<-chan "
		default:
			dir = "chan "
		}
		return dir + extractTypeFromExpression(expr.Value)
	case *ast.SelectorExpr:
		return extractTypeFromExpression(expr.X) + "." + expr.Sel.Name
	default:
		return "unknown"
	}
}
