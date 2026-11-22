package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type binding struct {
	key      string
	consumes bool // the case always returns
	where    string
}

type keySwitch struct {
	bindings []binding
}

// source is the parsed tui package.
type source struct {
	fset     *token.FileSet
	files    map[string][]byte // file name -> content
	funcs    map[string][]*ast.FuncDecl
	switches map[SwitchRef]*keySwitch
	keySw    map[SwitchRef]bool // switches on msg.String() or key
	consts   map[string]string
}

func load(dir string) (*source, error) {
	src := &source{fset: token.NewFileSet(), files: map[string][]byte{}, funcs: map[string][]*ast.FuncDecl{},
		switches: map[SwitchRef]*keySwitch{}, keySw: map[SwitchRef]bool{}, consts: map[string]string{}}
	paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		if err := src.addFile(p); err != nil {
			return nil, err
		}
	}
	if len(src.funcs) == 0 {
		return nil, fmt.Errorf("no Go source in %s", dir)
	}
	return src, nil
}

func (s *source) addFile(path string) error {
	data, err := os.ReadFile(path) //nolint:gosec // source files of the directory the caller names
	if err != nil {
		return err
	}
	name := filepath.Base(path)
	f, err := parser.ParseFile(s.fset, name, data, 0)
	if err != nil {
		return err
	}
	s.files[name] = data
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			s.funcs[d.Name.Name] = append(s.funcs[d.Name.Name], d)
			s.collectSwitches(d)
		case *ast.GenDecl:
			if d.Tok == token.CONST {
				s.addConsts(d)
			}
		}
	}
	return nil
}

// addConsts records the string constants of a const declaration.
func (s *source) addConsts(d *ast.GenDecl) {
	for _, spec := range d.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for i, name := range vs.Names {
			if i >= len(vs.Values) {
				break
			}
			if v, ok := stringLit(vs.Values[i]); ok {
				s.consts[name.Name] = v
			}
		}
	}
}

// stringLit returns the value of a string literal expression.
func stringLit(e ast.Node) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	v, err := strconv.Unquote(lit.Value)
	return v, err == nil
}

func (s *source) where(n ast.Node) string {
	p := s.fset.Position(n.Pos())
	return fmt.Sprintf("%s:%d", p.Filename, p.Line)
}

// collectSwitches records, in source order, every switch of fn that has
// string-literal cases.
func (s *source) collectSwitches(fn *ast.FuncDecl) {
	if fn.Body == nil {
		return
	}
	idx := 0
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		sw, ok := n.(*ast.SwitchStmt)
		if !ok {
			return true
		}
		ks := &keySwitch{}
		for _, st := range sw.Body.List {
			ks.bindings = append(ks.bindings, s.caseBindings(st.(*ast.CaseClause))...)
		}
		if len(ks.bindings) == 0 {
			return true
		}
		ref := SwitchRef{fn.Name.Name, idx}
		s.switches[ref] = ks
		s.keySw[ref] = isKeyTag(sw.Tag)
		idx++
		return true
	})
}

func (s *source) caseBindings(cc *ast.CaseClause) []binding {
	consumes := false
	if len(cc.Body) > 0 {
		_, consumes = cc.Body[len(cc.Body)-1].(*ast.ReturnStmt)
	}
	var out []binding
	for _, e := range cc.List {
		if k, ok := stringLit(e); ok {
			out = append(out, binding{key: k, consumes: consumes, where: s.where(e)})
		}
	}
	return out
}

// isKeyTag reports whether a switch tag is a key: msg.String() or key.
func isKeyTag(e ast.Expr) bool {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name == "key"
	case *ast.CallExpr:
		sel, ok := t.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "String" || len(t.Args) != 0 {
			return false
		}
		id, ok := sel.X.(*ast.Ident)
		return ok && id.Name == "msg"
	}
	return false
}

func (s *source) text(n ast.Node) string {
	pos, end := s.fset.Position(n.Pos()), s.fset.Position(n.End())
	return string(s.files[pos.Filename][pos.Offset:end.Offset])
}

// hintLiterals returns the string literals h selects and where the first is.
func (s *source) hintLiterals(h HintRef) ([]string, string, error) {
	fns := s.funcs[h.Func]
	if len(fns) != 1 {
		return nil, "", fmt.Errorf("hint function %s: found %d definitions, want 1", h.Func, len(fns))
	}
	var lits []string
	where := ""
	for _, node := range s.hintScope(fns[0], h.Clause) {
		ast.Inspect(node, func(n ast.Node) bool {
			v, ok := stringLit(n)
			if !ok || !strings.Contains(v, h.Contains) {
				return true
			}
			if where == "" {
				where = s.where(n)
			}
			lits = append(lits, v)
			return true
		})
	}
	if len(lits) == 0 {
		return nil, "", fmt.Errorf("hint %+v matches no string literal", h)
	}
	return lits, where, nil
}

// hintScope is fn's body, or the statements of the case clauses whose
// condition contains clause ("default" for the default clause).
func (s *source) hintScope(fn *ast.FuncDecl, clause string) []ast.Node {
	if clause == "" {
		return []ast.Node{fn.Body}
	}
	var scope []ast.Node
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if cc, ok := n.(*ast.CaseClause); ok && s.clauseMatches(cc, clause) {
			for _, st := range cc.Body {
				scope = append(scope, st)
			}
		}
		return true
	})
	return scope
}

func (s *source) clauseMatches(cc *ast.CaseClause, clause string) bool {
	if clause == "default" {
		return cc.List == nil
	}
	for _, e := range cc.List {
		if strings.Contains(s.text(e), clause) {
			return true
		}
	}
	return false
}
