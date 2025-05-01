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
	var found bool
	var err error
	var fields []Field

	ast.Inspect(node, func(n ast.Node) bool {
		if typeSpec, ok := n.(*ast.TypeSpec); ok {
			if typeSpec.Name != nil && typeSpec.Name.String() == typeName {
				found = true

				switch t := typeSpec.Type.(type) {
				case *ast.StructType:
					for _, f := range t.Fields.List {
						if len(f.Names) == 0 {
							// Embedded field
							children := getChildrenFields(f.Type)
							field := Field{
								Name:     extractTypeFromExpression(f.Type),
								Type:     extractTypeFromExpression(f.Type),
								Children: children,
							}
							fields = append(fields, field)
						} else {
							for _, n := range f.Names {
								children := getChildrenFields(f.Type)
								field := Field{
									Name:     n.Name,
									Type:     extractTypeFromExpression(f.Type),
									Children: children,
								}
								fields = append(fields, field)
							}
						}
					}
				case *ast.MapType:
					keyField := Field{
						Name: "key",
						Type: extractTypeFromExpression(t.Key),
					}
					valueField := Field{
						Name: "value",
						Type: extractTypeFromExpression(t.Value),
					}
					fields = append(fields, keyField, valueField)
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

func getChildrenFields(expr ast.Expr) *[]Field {
	switch t := expr.(type) {
	case *ast.StructType:
		var children []Field
		for _, f := range t.Fields.List {
			if len(f.Names) == 0 {
				field := Field{
					Name:     extractTypeFromExpression(f.Type),
					Type:     extractTypeFromExpression(f.Type),
					Children: getChildrenFields(f.Type),
				}
				children = append(children, field)
			} else {
				for _, n := range f.Names {
					field := Field{
						Name:     n.Name,
						Type:     extractTypeFromExpression(f.Type),
						Children: getChildrenFields(f.Type),
					}
					children = append(children, field)
				}
			}
		}
		return &children
	case *ast.Ident:
		if obj := t.Obj; obj != nil {
			if typeSpec, ok := obj.Decl.(*ast.TypeSpec); ok {
				if structType, ok := typeSpec.Type.(*ast.StructType); ok {
					return getChildrenFields(structType)
				}
			}
		}
		return nil
	default:
		return nil
	}
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
