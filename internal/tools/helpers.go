package tools

import (
	"context"
	"go/ast"
	"go/token"
	"go/types"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/go/packages"
)

var packageCache = struct {
	sync.RWMutex

	pkgs map[string][]*packages.Package
}{pkgs: make(map[string][]*packages.Package)}

// loadPackagesWithCache loads packages with directory and mode-based caching.
func loadPackagesWithCache(ctx context.Context, dir string, mode packages.LoadMode) ([]*packages.Package, error) {
	cacheKey := dir + "|" + strconv.FormatInt(int64(mode), 10)

	packageCache.RLock()
	cachedPkgs, exists := packageCache.pkgs[cacheKey]
	packageCache.RUnlock()

	if exists {
		return cachedPkgs, nil
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

	// Cache the packages
	packageCache.Lock()
	packageCache.pkgs[cacheKey] = pkgs
	packageCache.Unlock()

	return pkgs, nil
}

var fileLinesCache = struct {
	sync.RWMutex

	data map[string][]string
}{
	data: make(map[string][]string),
}

func getFileLines(fset *token.FileSet, file *ast.File) []string {
	filename := fset.File(file.Pos()).Name()

	// check cache
	fileLinesCache.RLock()
	lines, ok := fileLinesCache.data[filename]
	fileLinesCache.RUnlock()

	if ok {
		return lines
	}

	// load file
	src, err := os.ReadFile(filename)
	if err != nil {
		return []string{}
	}

	lines = strings.Split(string(src), "\n")

	// cache it
	fileLinesCache.Lock()
	fileLinesCache.data[filename] = lines
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
