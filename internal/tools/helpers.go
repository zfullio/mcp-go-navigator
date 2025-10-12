package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/pmezard/go-difflib/difflib"
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

const packageSuggestionLimit = 5

func normalizePackagePath(pkg *packages.Package) string {
	if pkg == nil {
		return ""
	}

	if pkg.PkgPath != "" {
		return pkg.PkgPath
	}

	return pkg.Name
}

func relativePath(baseDir, filename string) string {
	if filename == "" {
		return ""
	}

	absFile := filename
	if !filepath.IsAbs(absFile) {
		if abs, err := filepath.Abs(absFile); err == nil {
			absFile = abs
		} else {
			return filepath.ToSlash(filename)
		}
	}

	absBase := baseDir
	if absBase == "" {
		absBase = "."
	}

	if !filepath.IsAbs(absBase) {
		if abs, err := filepath.Abs(absBase); err == nil {
			absBase = abs
		} else {
			return filepath.ToSlash(filename)
		}
	}

	rel, err := filepath.Rel(absBase, absFile)
	if err != nil {
		return filepath.ToSlash(filename)
	}

	return filepath.ToSlash(rel)
}

func resolveFilePath(pkg *packages.Package, inputDir string, fileIndex int, file *ast.File) string {
	var absPath string

	if f := pkg.Fset.File(file.Pos()); f != nil {
		absPath = f.Name()
	}

	if absPath == "" {
		if len(pkg.CompiledGoFiles) > fileIndex {
			absPath = pkg.CompiledGoFiles[fileIndex]
		} else if len(pkg.GoFiles) > fileIndex {
			absPath = pkg.GoFiles[fileIndex]
		}
	}

	if absPath == "" {
		return ""
	}

	relPath := relativePath(inputDir, absPath)
	if relPath == "" {
		return filepath.ToSlash(absPath)
	}

	return relPath
}

func walkPackageFiles(ctx context.Context, pkgs []*packages.Package, dir string, fn func(pkg *packages.Package, file *ast.File, relPath string, fileIndex int) error) error {
	for _, pkg := range pkgs {
		for i, file := range pkg.Syntax {
			if shouldStop(ctx) {
				return context.Canceled
			}

			relPath := resolveFilePath(pkg, dir, i, file)

			err := fn(pkg, file, relPath, i)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func validatePagination(limit, offset int) error {
	if limit < 0 {
		return errors.New("limit must be >= 0")
	}

	if offset < 0 {
		return errors.New("offset must be >= 0")
	}

	return nil
}

func applyPagination(records []locationRecord, offset, limit int) (int, []locationRecord) {
	total := len(records)

	if offset < 0 {
		offset = 0
	}

	if offset > total {
		offset = total
	}

	return offset, paginateLocationRecords(records, offset, limit)
}

func loadFilteredPackages(ctx context.Context, dir string, mode packages.LoadMode, requested, tool string) ([]*packages.Package, []*packages.Package, error) {
	pkgs, err := loadPackagesWithCache(ctx, dir, mode)
	if err != nil {
		logError(tool, err, "failed to load packages")

		return nil, nil, err
	}

	filtered, err := filterPackagesByRequest(pkgs, requested)
	if err != nil {
		return nil, nil, err
	}

	return pkgs, filtered, nil
}

func filterPackagesByRequest(pkgs []*packages.Package, requested string) ([]*packages.Package, error) {
	if requested == "" {
		return pkgs, nil
	}

	var filtered []*packages.Package

	availableSet := make(map[string]struct{})
	available := make([]string, 0, len(pkgs))

	for _, pkg := range pkgs {
		key := normalizePackagePath(pkg)
		if key != "" {
			if _, seen := availableSet[key]; !seen {
				available = append(available, key)
				availableSet[key] = struct{}{}
			}
		}

		if key == requested || pkg.Name == requested {
			filtered = append(filtered, pkg)
		}
	}

	if len(filtered) > 0 {
		return filtered, nil
	}

	sort.Strings(available)

	if len(available) > packageSuggestionLimit {
		available = available[:packageSuggestionLimit]
	}

	suggestion := ""
	if len(available) > 0 {
		suggestion = "; available packages include: " + strings.Join(available, ", ")
	}

	return nil, fmt.Errorf("package %q not found%s", requested, suggestion)
}

func collectSymbols(file *ast.File, fset *token.FileSet, pkgPath, relPath string) []Symbol {
	if file == nil || fset == nil {
		return nil
	}

	if pkgPath == "" && file.Name != nil {
		pkgPath = file.Name.Name
	}

	symbols := make([]Symbol, 0)

	ast.Inspect(file, func(n ast.Node) bool {
		switch decl := n.(type) {
		case *ast.FuncDecl:
			symbols = append(symbols, Symbol{
				Kind:     "func",
				Name:     decl.Name.Name,
				Package:  pkgPath,
				File:     relPath,
				Line:     fset.Position(decl.Pos()).Line,
				Exported: decl.Name.IsExported(),
			})
		case *ast.TypeSpec:
			line := fset.Position(decl.Pos()).Line
			exported := decl.Name.IsExported()

			switch t := decl.Type.(type) {
			case *ast.StructType:
				symbols = append(symbols, Symbol{
					Kind:     "struct",
					Name:     decl.Name.Name,
					Package:  pkgPath,
					File:     relPath,
					Line:     line,
					Exported: exported,
				})
			case *ast.InterfaceType:
				symbols = append(symbols, Symbol{
					Kind:     "interface",
					Name:     decl.Name.Name,
					Package:  pkgPath,
					File:     relPath,
					Line:     line,
					Exported: exported,
				})

				if t.Methods != nil {
					for _, m := range t.Methods.List {
						if len(m.Names) == 0 {
							continue
						}

						name := m.Names[0]
						symbols = append(symbols, Symbol{
							Kind:     "method",
							Name:     decl.Name.Name + "." + name.Name,
							Package:  pkgPath,
							File:     relPath,
							Line:     fset.Position(m.Pos()).Line,
							Exported: name.IsExported(),
						})
					}
				}
			default:
				symbols = append(symbols, Symbol{
					Kind:     "type",
					Name:     decl.Name.Name,
					Package:  pkgPath,
					File:     relPath,
					Line:     line,
					Exported: exported,
				})
			}
		case *ast.GenDecl:
			switch decl.Tok {
			case token.CONST, token.VAR:
				for _, spec := range decl.Specs {
					valueSpec, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}

					for _, name := range valueSpec.Names {
						symbols = append(symbols, Symbol{
							Kind:     strings.ToLower(decl.Tok.String()),
							Name:     name.Name,
							Package:  pkgPath,
							File:     relPath,
							Line:     fset.Position(name.Pos()).Line,
							Exported: name.IsExported(),
						})
					}
				}
			}
		}

		return true
	})

	return symbols
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

	if a.Pos() != token.NoPos && a.Pos() == b.Pos() {
		return true
	}

	pkgA, pkgB := a.Pkg(), b.Pkg()

	if pkgA != nil && pkgB != nil {
		if pkgA.Path() != pkgB.Path() {
			return false
		}
	} else if pkgA != pkgB {
		return false
	}

	if a.Name() == b.Name() {
		typA, typB := a.Type(), b.Type()
		if typA != nil && typB != nil {
			if types.Identical(typA, typB) {
				return true
			}

			if typA.String() == typB.String() {
				return true
			}
		}
	}

	if types.ObjectString(a, nil) == types.ObjectString(b, nil) {
		return true
	}

	return false
}

// Helper functions for interface comparison.
func sameInterface(a, b *types.Interface) bool {
	if a.NumMethods() != b.NumMethods() {
		return false
	}

	// Compare methods between the two interfaces
	for i := range a.NumMethods() {
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
	for i := range target.NumMethods() {
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

type locationRecord struct {
	File    string
	Line    int
	Snippet string
}

func appendDefinition(out *[]locationRecord, dir string, fset *token.FileSet, pos token.Pos, fileFilter string) {
	posn := fset.Position(pos)
	if posn.Filename == "" {
		return
	}

	if fileFilter != "" && !strings.HasSuffix(posn.Filename, fileFilter) {
		return
	}

	rel := relativePath(dir, posn.Filename)
	lines := getFileLinesFromPath(posn.Filename)
	snippet := extractSnippet(lines, posn.Line)
	*out = append(*out, locationRecord{File: rel, Line: posn.Line, Snippet: snippet})
}

func appendReference(out *[]locationRecord, dir string, absPath string, line int, snippet string) {
	rel := relativePath(dir, absPath)
	*out = append(*out, locationRecord{File: rel, Line: line, Snippet: snippet})
}

func sortLocationRecords(records []locationRecord) {
	sort.SliceStable(records, func(i, j int) bool {
		if records[i].File == records[j].File {
			if records[i].Line == records[j].Line {
				return records[i].Snippet < records[j].Snippet
			}

			return records[i].Line < records[j].Line
		}

		return records[i].File < records[j].File
	})
}

func paginateLocationRecords(records []locationRecord, offset, limit int) []locationRecord {
	if offset < 0 {
		offset = 0
	}

	if offset >= len(records) {
		return nil
	}

	end := len(records)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}

	return records[offset:end]
}

func makeReferenceGroups(records []locationRecord) []ReferenceGroup {
	if len(records) == 0 {
		return nil
	}

	groups := make([]ReferenceGroup, 0)
	index := make(map[string]int, len(records))

	for _, rec := range records {
		if idx, ok := index[rec.File]; ok {
			groups[idx].References = append(groups[idx].References, ReferenceEntry{
				Line:    rec.Line,
				Snippet: rec.Snippet,
			})

			continue
		}

		index[rec.File] = len(groups)

		groups = append(groups, ReferenceGroup{
			File: rec.File,
			References: []ReferenceEntry{{
				Line:    rec.Line,
				Snippet: rec.Snippet,
			}},
		})
	}

	return groups
}

func makeDefinitionGroups(records []locationRecord) []DefinitionGroup {
	if len(records) == 0 {
		return nil
	}

	groups := make([]DefinitionGroup, 0)
	index := make(map[string]int, len(records))

	for _, rec := range records {
		if idx, ok := index[rec.File]; ok {
			groups[idx].Definitions = append(groups[idx].Definitions, DefinitionEntry{
				Line:    rec.Line,
				Snippet: rec.Snippet,
			})

			continue
		}

		index[rec.File] = len(groups)

		groups = append(groups, DefinitionGroup{
			File: rec.File,
			Definitions: []DefinitionEntry{{
				Line:    rec.Line,
				Snippet: rec.Snippet,
			}},
		})
	}

	return groups
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

func diffFiles(oldData, newData []byte, rel string) string {
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(oldData)),
		B:        difflib.SplitLines(string(newData)),
		FromFile: "a/" + rel,
		ToFile:   "b/" + rel,
		Context:  3,
	}
	text, _ := difflib.GetUnifiedDiffString(diff)

	return text
}

func computeFunctionMetrics(ctx context.Context, fset *token.FileSet, fn *ast.FuncDecl) (lines int, maxNesting int, cyclomatic int) {
	if fn == nil || fn.Body == nil {
		return 0, 0, 0
	}

	start := fset.Position(fn.Pos()).Line

	end := fset.Position(fn.End()).Line
	if end >= start {
		lines = end - start
	}

	visitor := &ComplexityVisitor{
		Ctx: ctx, Fset: fset, Nesting: 0, MaxNesting: 0, Cyclomatic: 1,
	}
	ast.Walk(visitor, fn.Body)

	return lines, visitor.MaxNesting, visitor.Cyclomatic
}

// filterSymbols applies user-defined filters (kind, name, exportedOnly).
func filterSymbols(symbols []Symbol, filter ReadGoFileFilter) []Symbol {
	if len(symbols) == 0 {
		return symbols
	}

	var filtered []Symbol

	for _, s := range symbols {
		if len(filter.SymbolKinds) > 0 && !contains(filter.SymbolKinds, s.Kind) {
			continue
		}

		if filter.NameContains != "" && !strings.Contains(strings.ToLower(s.Name), strings.ToLower(filter.NameContains)) {
			continue
		}

		if filter.ExportedOnly && !ast.IsExported(s.Name) {
			continue
		}

		filtered = append(filtered, s)
	}

	return filtered
}

func contains(list []string, val string) bool {
	for _, v := range list {
		if v == val {
			return true
		}
	}

	return false
}
