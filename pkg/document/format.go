package document

import (
	"fmt"
	"go/ast"
	"go/doc"
	"go/token"
	"strings"
	"unicode"
)

func isExported(name string) bool {
	if name == "" {
		return false
	}
	return unicode.IsUpper(rune(name[0]))
}

func hasExportedName(names []string) bool {
	for _, name := range names {
		if isExported(name) {
			return true
		}
	}
	return false
}

func getFunctionSignature(fset *token.FileSet, f *doc.Func) string {
	decl := f.Decl
	
	return formatFuncSignature(decl, "")
}

func getMethodSignature(fset *token.FileSet, m *doc.Func, receiverType string) string {
	decl := m.Decl
	
	return formatFuncSignature(decl, receiverType)
}

func formatFuncSignature(decl *ast.FuncDecl, receiverType string) string {
	name := decl.Name.Name
	typeSig := parseFuncType(decl.Type)
	
	signature := name
	
	if receiverType != "" {
		signature = fmt.Sprintf("(%s) %s", receiverType, signature)
	}
	
	signature += typeSig
	
	return signature
}

func parseFuncType(funcType *ast.FuncType) string {
	var params, results []string
	
	if funcType.Params != nil {
		for _, param := range funcType.Params.List {
			paramType := formatNode(param.Type)
			names := make([]string, 0, len(param.Names))
			for _, name := range param.Names {
				names = append(names, name.Name)
			}
			
			if len(names) > 0 {
				params = append(params, fmt.Sprintf("%s %s", strings.Join(names, ", "), paramType))
			} else {
				params = append(params, paramType)
			}
		}
	}
	
	if funcType.Results != nil {
		for _, result := range funcType.Results.List {
			resultType := formatNode(result.Type)
			names := make([]string, 0, len(result.Names))
			for _, name := range result.Names {
				names = append(names, name.Name)
			}
			
			if len(names) > 0 {
				results = append(results, fmt.Sprintf("%s %s", strings.Join(names, ", "), resultType))
			} else {
				results = append(results, resultType)
			}
		}
	}
	
	typeSig := fmt.Sprintf("(%s)", strings.Join(params, ", "))
	
	if len(results) > 0 {
		if len(results) == 1 && !strings.Contains(results[0], " ") {
			typeSig += " " + results[0]
		} else {
			typeSig += fmt.Sprintf(" (%s)", strings.Join(results, ", "))
		}
	}
	
	return typeSig
}

func formatNode(node ast.Node) string {
	switch n := node.(type) {
	case *ast.Ident:
		return n.Name
	case *ast.SelectorExpr:
		return formatNode(n.X) + "." + n.Sel.Name
	case *ast.StarExpr:
		return "*" + formatNode(n.X)
	case *ast.ArrayType:
		if n.Len == nil {
			return "[]" + formatNode(n.Elt)
		}
		return fmt.Sprintf("[%s]%s", formatNode(n.Len), formatNode(n.Elt))
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", formatNode(n.Key), formatNode(n.Value))
	case *ast.InterfaceType:
		if n.Methods == nil || len(n.Methods.List) == 0 {
			return "interface{}"
		}
		return "interface{...}"
	case *ast.FuncType:
		return "func" + parseFuncType(n)
	case *ast.ChanType:
		return "chan " + formatNode(n.Value)
	case *ast.StructType:
		if n.Fields == nil || len(n.Fields.List) == 0 {
			return "struct{}"
		}
		return "struct{...}"
	case *ast.BasicLit:
		return n.Value
	case *ast.Ellipsis:
		return "..." + formatNode(n.Elt)
	default:
		return "<?>"
	}
}

func extractStructFields(structType *ast.StructType) []string {
	if structType.Fields == nil {
		return nil
	}

	var fields []string
	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 || !isExported(field.Names[0].Name) {
			continue
		}

		fieldType := formatNode(field.Type)
		fieldName := field.Names[0].Name
		fieldStr := fmt.Sprintf("%s %s", fieldName, fieldType)

		if field.Tag != nil {
			tag := field.Tag.Value
			tag = strings.Trim(tag, "`")
			fieldStr += fmt.Sprintf(" `%s`", tag)
		}

		fields = append(fields, fieldStr)
	}

	return fields
}

func extractInterfaceMethods(interfaceType *ast.InterfaceType) []string {
	if interfaceType.Methods == nil {
		return nil
	}

	var methods []string
	for _, method := range interfaceType.Methods.List {
		if len(method.Names) == 0 {
			embeddedName := formatNode(method.Type)
			methods = append(methods, embeddedName)
			continue
		}

		if !isExported(method.Names[0].Name) {
			continue
		}

		methodName := method.Names[0].Name
		
		switch methodType := method.Type.(type) {
		case *ast.FuncType:
			sig := fmt.Sprintf("%s%s", methodName, parseFuncType(methodType))
			methods = append(methods, sig)
		default:
			methods = append(methods, fmt.Sprintf("%s %s", methodName, formatNode(method.Type)))
		}
	}

	return methods
}