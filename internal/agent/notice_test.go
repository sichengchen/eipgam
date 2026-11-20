package agent

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// A notice is written as a newNotice template, never as English in the
// code that gives it, so a new agent's advice can't miss the translation
// TestEveryNoticeIsTranslated asks for.
func TestNoticesAreTemplates(t *testing.T) {
	files, _ := filepath.Glob("*.go")
	prose := regexp.MustCompile(`[A-Za-z']+ [A-Za-z']+ [A-Za-z']+`)
	fset := token.NewFileSet()
	checked := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") || name == "notice.go" {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		check := func(n ast.Node) {
			checked++
			ast.Inspect(n, func(n ast.Node) bool {
				// newNotice is where a template is written
				if c, ok := n.(*ast.CallExpr); ok {
					if id, ok := c.Fun.(*ast.Ident); ok && id.Name == "newNotice" {
						return false
					}
				}
				if l, ok := n.(*ast.BasicLit); ok && l.Kind == token.STRING && prose.MatchString(l.Value) {
					t.Errorf("%s: a notice written in place, not as a newNotice: %s", fset.Position(l.Pos()), l.Value)
				}
				return true
			})
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.KeyValueExpr:
				if k, ok := n.Key.(*ast.Ident); ok && k.Name == "Notice" {
					check(n.Value)
				}
			case *ast.AssignStmt:
				for i, l := range n.Lhs {
					if s, ok := l.(*ast.SelectorExpr); ok && s.Sel.Name == "Notice" && i < len(n.Rhs) {
						check(n.Rhs[i])
					}
				}
			case *ast.FuncDecl:
				// the helpers a Notice says through
				if strings.Contains(strings.ToLower(n.Name.Name), "notice") || n.Name.Name == "notMirrored" {
					check(n)
				}
			}
			return true
		})
	}
	// 25 with the agents eipgam keeps
	if checked < 20 {
		t.Fatalf("checked only %d notices", checked)
	}
}
