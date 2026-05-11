package mutations

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
)

type Site struct {
	Index       int
	File        string
	Line        int
	Column      int
	StartOffset int
	EndOffset   int
	Original    string
	Mutant      string
	Category    string
	Description string
	FunctionID  string
}

type Function struct {
	ID        string
	Name      string
	StartLine int
	EndLine   int
	Text      string
}

func Discover(path string) ([]Site, []Function, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, content, 0)
	if err != nil {
		return nil, nil, err
	}
	functions := extractFunctions(fset, content, file)
	var sites []Site
	ast.Inspect(file, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.BasicLit:
			if n.Kind == token.INT {
				sites = addLiteralMutation(sites, fset, path, n.Pos(), n.End(), n.Value, "constant", content, functions)
			}
		case *ast.Ident:
			if n.Name == "true" || n.Name == "false" {
				sites = addTokenMutation(sites, fset, path, n.Pos(), n.End(), n.Name, boolMutant(n.Name), "boolean", content, functions)
			}
		case *ast.BinaryExpr:
			if mutant, category, ok := binaryMutant(n.Op); ok {
				pos := n.OpPos
				sites = addTokenMutation(sites, fset, path, pos, pos+token.Pos(len(n.Op.String())), n.Op.String(), mutant, category, content, functions)
			}
		}
		return true
	})
	sort.SliceStable(sites, func(i, j int) bool {
		if sites[i].Line == sites[j].Line {
			return sites[i].Column < sites[j].Column
		}
		return sites[i].Line < sites[j].Line
	})
	for i := range sites {
		sites[i].Index = i
	}
	return sites, functions, nil
}

func Apply(content string, site Site) string {
	return content[:site.StartOffset] + site.Mutant + content[site.EndOffset:]
}

func addLiteralMutation(sites []Site, fset *token.FileSet, path string, start, end token.Pos, value, category string, content []byte, functions []Function) []Site {
	switch value {
	case "0":
		return addTokenMutation(sites, fset, path, start, end, "0", "1", category, content, functions)
	case "1":
		return addTokenMutation(sites, fset, path, start, end, "1", "0", category, content, functions)
	default:
		return sites
	}
}

func addTokenMutation(sites []Site, fset *token.FileSet, path string, start, end token.Pos, original, mutant, category string, content []byte, functions []Function) []Site {
	position := fset.Position(start)
	startOffset := position.Offset
	endOffset := fset.Position(end).Offset
	return append(sites, Site{
		File:        path,
		Line:        position.Line,
		Column:      position.Column,
		StartOffset: startOffset,
		EndOffset:   endOffset,
		Original:    original,
		Mutant:      mutant,
		Category:    category,
		Description: original + " -> " + mutant,
		FunctionID:  functionIDAtLine(functions, position.Line),
	})
}

func binaryMutant(op token.Token) (string, string, bool) {
	switch op {
	case token.ADD:
		return "-", "arithmetic", true
	case token.SUB:
		return "+", "arithmetic", true
	case token.MUL:
		return "/", "arithmetic", true
	case token.GTR:
		return ">=", "comparison", true
	case token.GEQ:
		return ">", "comparison", true
	case token.LSS:
		return "<=", "comparison", true
	case token.LEQ:
		return "<", "comparison", true
	case token.EQL:
		return "!=", "equality", true
	case token.NEQ:
		return "==", "equality", true
	case token.LAND:
		return "||", "logical", true
	case token.LOR:
		return "&&", "logical", true
	default:
		return "", "", false
	}
}

func boolMutant(value string) string {
	if value == "true" {
		return "false"
	}
	return "true"
}

func extractFunctions(fset *token.FileSet, content []byte, file *ast.File) []Function {
	var functions []Function
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		start := fset.Position(fn.Pos())
		end := fset.Position(fn.End())
		functions = append(functions, Function{
			ID:        functionID(fn),
			Name:      functionName(fn),
			StartLine: start.Line,
			EndLine:   end.Line,
			Text:      string(content[start.Offset:end.Offset]),
		})
	}
	return functions
}

func functionID(fn *ast.FuncDecl) string {
	return "func/" + functionName(fn)
}

func functionName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	return receiverName(fn.Recv.List[0].Type) + "." + fn.Name.Name
}

func receiverName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return receiverName(t.X)
	case *ast.IndexExpr:
		return receiverName(t.X)
	case *ast.IndexListExpr:
		return receiverName(t.X)
	default:
		return "receiver"
	}
}

func functionIDAtLine(functions []Function, line int) string {
	for _, fn := range functions {
		if line >= fn.StartLine && line <= fn.EndLine {
			return fn.ID
		}
	}
	return ""
}
