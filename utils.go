package main

import (
	"go/ast"
	"path/filepath"
	"strings"
)

func joinLinuxPath(elem ...string) string {
	joinedPath := filepath.Join(elem...)

	joinedPath = strings.ReplaceAll(joinedPath, "\\", "/")

	return joinedPath
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

func in(target string, list []string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}
