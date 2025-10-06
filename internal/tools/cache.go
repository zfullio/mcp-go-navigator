package tools

import (
	"context"
	"os"
	"strconv"
	"sync"
	"time"

	"golang.org/x/tools/go/packages"
)

// PackageCacheItem represents a cached package with its timestamp.
type PackageCacheItem struct {
	Packages    []*packages.Package
	LastAccess  time.Time
	FileModTime map[string]time.Time
}

var packageCache = struct {
	sync.RWMutex

	pkgs map[string]PackageCacheItem
}{pkgs: make(map[string]PackageCacheItem)}

// loadPackagesWithCache loads Go packages and caches them by (dir, mode),
// automatically invalidating cache when any source file was modified.
func loadPackagesWithCache(ctx context.Context, dir string, mode packages.LoadMode) ([]*packages.Package, error) {
	cacheKey := dir + "|" + strconv.FormatInt(int64(mode), 10)

	packageCache.RLock()
	item, exists := packageCache.pkgs[cacheKey]
	packageCache.RUnlock()

	// Проверяем, не изменились ли файлы
	if exists && !isPackageModified(item.FileModTime) {
		// Обновляем время доступа
		packageCache.Lock()

		item.LastAccess = time.Now()
		packageCache.pkgs[cacheKey] = item
		packageCache.Unlock()

		return item.Packages, nil
	}

	// Если кеш отсутствует или устарел — перезагружаем
	cfg := &packages.Config{
		Mode:    mode,
		Dir:     dir,
		Context: ctx,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, err
	}

	// Сохраняем времена модификации файлов
	fileModTimes := make(map[string]time.Time)

	for _, pkg := range pkgs {
		for _, f := range pkg.CompiledGoFiles {
			if st, err := os.Stat(f); err == nil {
				fileModTimes[f] = st.ModTime()
			}
		}
	}

	packageCache.Lock()
	packageCache.pkgs[cacheKey] = PackageCacheItem{
		Packages:    pkgs,
		LastAccess:  time.Now(),
		FileModTime: fileModTimes,
	}
	packageCache.Unlock()

	return pkgs, nil
}

// isPackageModified returns true if any file in the cached package set has changed.
func isPackageModified(stored map[string]time.Time) bool {
	for path, oldTime := range stored {
		st, err := os.Stat(path)
		if err != nil {
			// Если файл удалён — считаем, что пакет изменился
			return true
		}

		if st.ModTime().After(oldTime) {
			return true
		}
	}

	return false
}
