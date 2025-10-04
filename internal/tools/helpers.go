package tools

import (
	"context"
	"go/ast"
	"go/token"
	"os"
	"strconv"
	"strings"
	"sync"

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

func ctxCancelled(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

func extractSnippet(lines []string, line int) string {
	if line-1 < len(lines) && line-1 >= 0 {
		return strings.TrimSpace(lines[line-1])
	}

	return ""
}
