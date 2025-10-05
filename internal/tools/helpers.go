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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/go/packages"
)

// PackageCacheItem represents a cached package with its timestamp.
type PackageCacheItem struct {
	Packages   []*packages.Package
	LastAccess time.Time
}

var packageCache = struct {
	sync.RWMutex

	pkgs map[string]PackageCacheItem
}{pkgs: make(map[string]PackageCacheItem)}

// loadPackagesWithCache loads packages with directory and mode-based caching.
func loadPackagesWithCache(ctx context.Context, dir string, mode packages.LoadMode) ([]*packages.Package, error) {
	cacheKey := dir + "|" + strconv.FormatInt(int64(mode), 10)

	packageCache.RLock()
	item, exists := packageCache.pkgs[cacheKey]
	packageCache.RUnlock()

	if exists {
		// Update last access time
		packageCache.Lock()

		item.LastAccess = time.Now()
		packageCache.pkgs[cacheKey] = item
		packageCache.Unlock()

		return item.Packages, nil
	}

	cfg := &packages.Config{
		Mode:    mode,
		Dir:     dir,
		Context: ctx,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, err
	}

	// Cache the packages with access time
	packageCache.Lock()
	packageCache.pkgs[cacheKey] = PackageCacheItem{
		Packages:   pkgs,
		LastAccess: time.Now(),
	}
	packageCache.Unlock()

	return pkgs, nil
}

// FileLinesCacheItem represents a cached file lines entry with timestamp.
type FileLinesCacheItem struct {
	Lines      []string
	LastAccess time.Time
}

var fileLinesCache = struct {
	sync.RWMutex

	data map[string]FileLinesCacheItem
}{
	data: make(map[string]FileLinesCacheItem),
}

func getFileLines(fset *token.FileSet, file *ast.File) []string {
	filename := fset.File(file.Pos()).Name()

	// check cache
	fileLinesCache.RLock()
	item, ok := fileLinesCache.data[filename]
	fileLinesCache.RUnlock()

	if ok {
		// Update last access time
		fileLinesCache.Lock()

		item.LastAccess = time.Now()
		fileLinesCache.data[filename] = item
		fileLinesCache.Unlock()

		return item.Lines
	}

	// load file
	src, err := os.ReadFile(filename)
	if err != nil {
		return []string{}
	}

	lines := strings.Split(string(src), "\n")

	// cache it with access time
	fileLinesCache.Lock()
	fileLinesCache.data[filename] = FileLinesCacheItem{
		Lines:      lines,
		LastAccess: time.Now(),
	}
	fileLinesCache.Unlock()

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

// isDeadCandidate определяет, стоит ли вообще рассматривать объект как "мёртвый" кандидат.
func isDeadCandidate(ident *ast.Ident, obj types.Object) bool {
	name := ident.Name

	// --- очевидные исключения ---
	if name == "_" || name == "init" || name == "main" {
		return false
	}

	if ast.IsExported(name) {
		return false // экспортируемые символы не считаем "мёртвыми"
	}

	if strings.HasSuffix(name, "_test") {
		return false // тестовые функции
	}

	// --- интересуют только "реальные" сущности ---
	switch obj.(type) {
	case *types.Var, *types.Const, *types.TypeName, *types.Func:
		// ok
	default:
		return false
	}

	// --- уточняем для функций ---
	if fn, ok := obj.(*types.Func); ok {
		if sig, ok := fn.Type().(*types.Signature); ok && sig.Recv() != nil {
			// это метод
			// неэкспортируемые методы анализируем, экспортируемые — пропускаем
			if ast.IsExported(fn.Name()) {
				return false
			}
			// иначе — кандидат
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

// cleanupCache removes cache entries older than the specified duration.
func cleanupCache(maxAge time.Duration) {
	packageCache.Lock()
	defer packageCache.Unlock()

	now := time.Now()
	for key, item := range packageCache.pkgs {
		if now.Sub(item.LastAccess) > maxAge {
			delete(packageCache.pkgs, key)
		}
	}
}

// startCacheCleanup starts a background goroutine that periodically cleans up old cache entries.
func startCacheCleanup(interval time.Duration, maxAge time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			cleanupCache(maxAge)
		}
	}()
}

// FileLinesCacheCleanup removes old file lines cache entries.
func cleanupFileLinesCache(maxAge time.Duration) {
	fileLinesCache.Lock()
	defer fileLinesCache.Unlock()

	now := time.Now()
	for key, item := range fileLinesCache.data {
		if now.Sub(item.LastAccess) > maxAge {
			delete(fileLinesCache.data, key)
		}
	}
}

// startFileLinesCacheCleanup starts a background goroutine that periodically cleans up old file lines cache entries.
func startFileLinesCacheCleanup(interval time.Duration, maxAge time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			cleanupFileLinesCache(maxAge)
		}
	}()
}

// init initializes the cache cleanup processes.
func init() {
	// Clean up package cache entries older than 30 minutes every 10 minutes
	startCacheCleanup(10*time.Minute, 30*time.Minute)

	// Clean up file lines cache entries older than 30 minutes every 10 minutes
	startFileLinesCacheCleanup(10*time.Minute, 30*time.Minute)
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

func appendReference(out *[]Reference, dir string, fset *token.FileSet, absPath string, line int, snippet string) {
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
