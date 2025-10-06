package tools

import (
	"bytes"
	"context"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/go/packages"
)

func getFileLines(fset *token.FileSet, file *ast.File) []string {
	filename := fset.File(file.Pos()).Name()

	// Check cache
	fileLinesCache.RLock()
	item, ok := fileLinesCache.data[filename]
	fileLinesCache.RUnlock()

	if ok {
		// Check if the file has changed on disk
		if st, err := os.Stat(filename); err == nil {
			if st.ModTime().Equal(item.ModTime) {
				// File hasn't changed - return cache
				fileLinesCache.Lock()

				item.LastAccess = time.Now()
				fileLinesCache.data[filename] = item
				fileLinesCache.Unlock()

				return item.Lines
			}
		}
	}

	// File changed or not cached - read again
	src, err := os.ReadFile(filename)
	if err != nil {
		return []string{}
	}

	lines := strings.Split(string(src), "\n")

	var modTime time.Time
	if st, err := os.Stat(filename); err == nil {
		modTime = st.ModTime()
	}

	fileLinesCache.Lock()
	fileLinesCache.data[filename] = FileLinesCacheItem{
		Lines:      lines,
		LastAccess: time.Now(),
		ModTime:    modTime,
	}
	fileLinesCache.Unlock()

	// Add file to watcher to track changes
	_ = addFileToWatch(filename, filename) // Using filename as a simple cache key for file lines cache

	return lines
}

func getFileLinesFromPath(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	return strings.Split(string(data), "\n")
}

func extractSnippet(lines []string, line int) string {
	if line-1 < len(lines) && line-1 >= 0 {
		return strings.TrimSpace(lines[line-1])
	}

	return ""
}

func objStringKind(obj types.Object) string {
	switch obj.(type) {
	case *types.Func:
		return "func"
	case *types.Var:
		return "var"
	case *types.Const:
		return "const"
	case *types.TypeName:
		return "type"
	case *types.PkgName:
		return "package"
	default:
		return "unknown"
	}
}

func shouldStop(ctx context.Context) bool {
	return ctx.Err() != nil
}

func fail[T any](out T, err error) (*mcp.CallToolResult, T, error) {
	if err != nil {
		log.Printf("[go-navigator] error: %v", err)
	}

	return nil, out, err
}

func symbolPos(pkg *packages.Package, n ast.Node) token.Position {
	return pkg.Fset.Position(n.Pos())
}

func safeWriteFile(path string, data []byte) error {
	tmp := path + ".tmp"

	err := os.WriteFile(tmp, data, 0o644)
	if err != nil {
		return err
	}

	return os.Rename(tmp, path)
}

// isDeadCandidate determines whether to consider an object as a "dead" candidate at all.
func isDeadCandidate(ident *ast.Ident, obj types.Object) bool {
	name := ident.Name

	// --- obvious exclusions ---
	if name == "_" || name == "init" || name == "main" {
		return false
	}

	if ast.IsExported(name) {
		return false // don't consider exported symbols as "dead"
	}

	if strings.HasSuffix(name, "_test") {
		return false // test functions
	}

	// --- only interested in "real" entities ---
	switch obj.(type) {
	case *types.Var, *types.Const, *types.TypeName, *types.Func:
		// ok
	default:
		return false
	}

	// --- refine for functions ---
	if fn, ok := obj.(*types.Func); ok {
		if sig, ok := fn.Type().(*types.Signature); ok && sig.Recv() != nil {
			// this is a method
			// analyze unexported methods, skip exported ones
			if ast.IsExported(fn.Name()) {
				return false
			}
			// otherwise - candidate
		}
	}

	return true
}

func sameObject(a, b types.Object) bool {
	if a == nil || b == nil {
		return false
	}

	if a == b {
		return true
	}

	return a.Pkg() == b.Pkg() && a.Pos() == b.Pos()
}

// Helper functions for interface comparison.
func sameInterface(a, b *types.Interface) bool {
	if a.NumMethods() != b.NumMethods() {
		return false
	}

	// Compare methods between the two interfaces
	for i := 0; i < a.NumMethods(); i++ {
		methodA := a.Method(i)
		found := false

		for j := 0; j < b.NumMethods(); j++ {
			methodB := b.Method(j)
			if methodA.Name() == methodB.Name() {
				if types.Identical(methodA.Type(), methodB.Type()) {
					found = true

					break
				}
			}
		}

		if !found {
			return false
		}
	}

	return true
}

func interfaceExtends(impl, target *types.Interface) bool {
	// Check if impl extends target by having at least all of target's methods
	for i := 0; i < target.NumMethods(); i++ {
		targetMethod := target.Method(i)
		found := false

		for j := 0; j < impl.NumMethods(); j++ {
			implMethod := impl.Method(j)
			if targetMethod.Name() == implMethod.Name() &&
				types.Identical(targetMethod.Type(), implMethod.Type()) {
				found = true

				break
			}
		}

		if !found {
			return false
		}
	}

	return true
}

func findTargetObject(ctx context.Context, pkgs []*packages.Package, ident, kind string) types.Object {
	for _, pkg := range pkgs {
		if shouldStop(ctx) {
			return nil
		}

		if scope := pkg.Types.Scope(); scope != nil {
			if obj := scope.Lookup(ident); obj != nil {
				if kind == "" || objStringKind(obj) == kind {
					return obj
				}
			}
		}

		for id, def := range pkg.TypesInfo.Defs {
			if shouldStop(ctx) {
				return nil
			}

			if def != nil && id.Name == ident {
				if kind == "" || objStringKind(def) == kind {
					return def
				}
			}
		}
	}

	return nil
}

func appendDefinition(out *[]Definition, dir string, fset *token.FileSet, pos token.Pos, fileFilter string) {
	posn := fset.Position(pos)
	if posn.Filename == "" {
		return
	}

	if fileFilter != "" && !strings.HasSuffix(posn.Filename, fileFilter) {
		return
	}

	rel, _ := filepath.Rel(dir, posn.Filename)
	lines := getFileLinesFromPath(posn.Filename)
	snippet := extractSnippet(lines, posn.Line)
	*out = append(*out, Definition{File: rel, Line: posn.Line, Snippet: snippet})
}

func appendReference(out *[]Reference, dir string, absPath string, line int, snippet string) {
	rel, _ := filepath.Rel(dir, absPath)
	*out = append(*out, Reference{File: rel, Line: line, Snippet: snippet})
}

// astEqual compares two AST expressions structurally.
func astEqual(a, b ast.Expr) bool {
	if a == nil && b == nil {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	// Use the ast.Inspect function to walk both ASTs simultaneously for deep comparison
	// This is more efficient than formatting to strings
	return compareASTNodes(a, b)
}

// compareASTNodes compares two AST nodes recursively.
func compareASTNodes(a, b ast.Node) bool {
	if a == nil && b == nil {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	// Use type switches to compare the specific node types
	switch aVal := a.(type) {
	case *ast.Ident:
		if bVal, ok := b.(*ast.Ident); ok {
			return aVal.Name == bVal.Name
		}
	case *ast.BasicLit:
		if bVal, ok := b.(*ast.BasicLit); ok {
			return aVal.Kind == bVal.Kind && aVal.Value == bVal.Value
		}
	case *ast.BinaryExpr:
		if bVal, ok := b.(*ast.BinaryExpr); ok {
			return aVal.Op == bVal.Op &&
				compareASTNodes(aVal.X, bVal.X) &&
				compareASTNodes(aVal.Y, bVal.Y)
		}
	case *ast.CallExpr:
		if bVal, ok := b.(*ast.CallExpr); ok {
			return compareASTNodes(aVal.Fun, bVal.Fun) &&
				compareExprSlices(aVal.Args, bVal.Args)
		}
	case *ast.SelectorExpr:
		if bVal, ok := b.(*ast.SelectorExpr); ok {
			return compareASTNodes(aVal.X, bVal.X) &&
				compareASTNodes(aVal.Sel, bVal.Sel)
		}
	case *ast.ParenExpr:
		if bVal, ok := b.(*ast.ParenExpr); ok {
			return compareASTNodes(aVal.X, bVal.X)
		}
	case *ast.StarExpr:
		if bVal, ok := b.(*ast.StarExpr); ok {
			return compareASTNodes(aVal.X, bVal.X)
		}
	case *ast.TypeAssertExpr:
		if bVal, ok := b.(*ast.TypeAssertExpr); ok {
			return compareASTNodes(aVal.X, bVal.X) &&
				compareASTNodes(aVal.Type, bVal.Type)
		}
	case *ast.IndexExpr:
		if bVal, ok := b.(*ast.IndexExpr); ok {
			return compareASTNodes(aVal.X, bVal.X) &&
				compareASTNodes(aVal.Index, bVal.Index)
		}
	case *ast.SliceExpr:
		if bVal, ok := b.(*ast.SliceExpr); ok {
			return compareASTNodes(aVal.X, bVal.X) &&
				compareASTNodes(aVal.Low, bVal.Low) &&
				compareASTNodes(aVal.High, bVal.High) &&
				compareASTNodes(aVal.Max, bVal.Max)
		}
	case *ast.FuncLit:
		if bVal, ok := b.(*ast.FuncLit); ok {
			// For FuncLit, we'll do a basic comparison of type and body since
			// comparing functions properly is complex
			return compareASTNodes(aVal.Type, bVal.Type) &&
				compareASTNodes(aVal.Body, bVal.Body)
		}
	case *ast.CompositeLit:
		if bVal, ok := b.(*ast.CompositeLit); ok {
			return compareASTNodes(aVal.Type, bVal.Type) &&
				compareExprSlices(aVal.Elts, bVal.Elts)
		}
	case *ast.KeyValueExpr:
		if bVal, ok := b.(*ast.KeyValueExpr); ok {
			return compareASTNodes(aVal.Key, bVal.Key) &&
				compareASTNodes(aVal.Value, bVal.Value)
		}
	case *ast.UnaryExpr:
		if bVal, ok := b.(*ast.UnaryExpr); ok {
			return aVal.Op == bVal.Op &&
				compareASTNodes(aVal.X, bVal.X)
		}
	// Add other cases as needed for more comprehensive comparison
	// For this implementation, if it's not one of the known types,
	// we'll fall back to a string comparison approach which is less efficient
	// but covers all cases
	default:
		var bufA, bufB bytes.Buffer

		fset := token.NewFileSet()
		_ = format.Node(&bufA, fset, a)
		_ = format.Node(&bufB, fset, b)

		return bufA.String() == bufB.String()
	}

	return false
}

// compareExprSlices compares two slices of expressions.
func compareExprSlices(a, b []ast.Expr) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if !compareASTNodes(a[i], b[i]) {
			return false
		}
	}

	return true
}

// receiverName returns the receiver type name for a method if present.
// For example, for `func (s *TaskService) List()` returns "TaskService".
// If the function is not a method, returns an empty string.
func receiverName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return ""
	}

	recvType := fd.Recv.List[0].Type

	switch expr := recvType.(type) {
	case *ast.StarExpr:
		// Pointer to type, e.g. (*TaskService)
		if ident, ok := expr.X.(*ast.Ident); ok {
			return ident.Name
		}
	case *ast.Ident:
		// Direct type without pointer, e.g. TaskService
		return expr.Name
	}

	return ""
}

// exprString returns the string representation of an AST expression type (for struct fields).
func exprString(e ast.Expr) string {
	var buf bytes.Buffer

	err := format.Node(&buf, token.NewFileSet(), e)
	if err != nil {
		return ""
	}

	return buf.String()
}
