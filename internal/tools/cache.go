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

	// Check if files have changed
	if exists && !isPackageModified(item.FileModTime) {
		// Update access time
		packageCache.Lock()

		item.LastAccess = time.Now()
		packageCache.pkgs[cacheKey] = item
		packageCache.Unlock()

		return item.Packages, nil
	}

	// If cache is missing or outdated - reload
	cfg := &packages.Config{
		Mode:    mode,
		Dir:     dir,
		Context: ctx,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, err
	}

	// Save file modification times
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
			// If file is deleted - consider that the package has changed
			return true
		}

		if st.ModTime().After(oldTime) {
			return true
		}
	}

	return false
}

// FileLinesCacheItem represents a cached file lines entry with timestamp.
type FileLinesCacheItem struct {
	Lines      []string
	LastAccess time.Time
	ModTime    time.Time
}

var fileLinesCache = struct {
	sync.RWMutex

	data map[string]FileLinesCacheItem
}{
	data: make(map[string]FileLinesCacheItem),
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
