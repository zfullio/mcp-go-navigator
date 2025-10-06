package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"golang.org/x/tools/go/packages"
)

func makeCacheKey(dir string, mode packages.LoadMode) string {
	h := sha256.New()
	h.Write([]byte(dir))
	h.Write([]byte("|"))
	h.Write([]byte(strconv.FormatInt(int64(mode), 10)))

	return hex.EncodeToString(h.Sum(nil))
}

// PackageCacheItem represents a cached package with its timestamp.
type PackageCacheItem struct {
	Packages      []*packages.Package
	LastAccess    time.Time
	FileModTime   map[string]time.Time
	LastFileCheck time.Time     // Last time we checked file modification times
	CheckValidFor time.Duration // Time for which file check is valid (e.g., 5 seconds)
}

var packageCache = struct {
	sync.RWMutex

	pkgs map[string]PackageCacheItem
}{pkgs: make(map[string]PackageCacheItem)}

// loadPackagesWithCache loads Go packages and caches them by (dir, mode),
// automatically invalidating cache when any source file was modified.
func loadPackagesWithCache(ctx context.Context, dir string, mode packages.LoadMode) ([]*packages.Package, error) {
	cacheKey := makeCacheKey(dir, mode)

	packageCache.RLock()
	item, exists := packageCache.pkgs[cacheKey]
	packageCache.RUnlock()

	if exists {
		// Check if we should verify file modification times (e.g., only every 5 seconds)
		shouldCheckFiles := time.Since(item.LastFileCheck) > item.CheckValidFor
		modified := false

		if shouldCheckFiles {
			modified = isPackageModified(item.FileModTime)
			if !modified {
				// Update the file check time
				packageCache.Lock()
				item.LastFileCheck = time.Now()
				packageCache.pkgs[cacheKey] = item
				packageCache.Unlock()
			}
		}

		if !shouldCheckFiles || !modified {
			// Use cached data
			packageCache.Lock()
			item.LastAccess = time.Now()
			packageCache.pkgs[cacheKey] = item
			packageCache.Unlock()
			return item.Packages, nil
		}
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

	// Save file modification times and add files to watcher
	fileModTimes := make(map[string]time.Time)

	for _, pkg := range pkgs {
		// Track all directories that contain Go files
		dirsAdded := make(map[string]bool)

		for _, f := range pkg.CompiledGoFiles {
			if st, err := os.Stat(f); err == nil {
				fileModTimes[f] = st.ModTime()
				// Add file to watcher
				_ = addFileToWatch(f, cacheKey)

				// Add directory to watcher to track new files
				dir := filepath.Dir(f)
				if !dirsAdded[dir] {
					_ = AddDirToWatch(dir)
					dirsAdded[dir] = true
				}
			}
		}
	}

	packageCache.Lock()
	packageCache.pkgs[cacheKey] = PackageCacheItem{
		Packages:      pkgs,
		LastAccess:    time.Now(),
		FileModTime:   fileModTimes,
		LastFileCheck: time.Now(),
		CheckValidFor: 5 * time.Second, // Only check file modification every 5 seconds
	}
	packageCache.Unlock()

	return pkgs, nil
}

// isPackageModified returns true if any file in the cached package set has changed.
func isPackageModified(stored map[string]time.Time) bool {
	for path, oldTime := range stored {
		st, err := os.Stat(path)
		if err != nil {
			// If file doesn't exist (ENOENT) or we can't access it, consider that the package has changed
			if os.IsNotExist(err) {
				return true
			}
			// For other errors (permission denied, etc.), we still consider the file changed to be safe
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

// Global file watcher and tracking
var fileWatcher = struct {
	sync.RWMutex
	watcher *fsnotify.Watcher
	// Track which cache keys are associated with each file path
	fileToCacheKeys map[string]map[string]bool // filePath -> set of cacheKeys
}{
	fileToCacheKeys: make(map[string]map[string]bool),
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

// initFileWatcher initializes and starts the file system watcher
func initFileWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	fileWatcher.Lock()
	fileWatcher.watcher = watcher
	fileWatcher.Unlock()

	// Start goroutine to handle watcher events
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
					// Handle file/directory changes
					if event.Op&fsnotify.Create != 0 {
						// Check if this is a new directory
						if fileInfo, err := os.Stat(event.Name); err == nil && fileInfo.IsDir() {
							// Add the new directory to watcher so we can track files in it
							_ = AddDirToWatch(event.Name)
						}
					}
					// File was modified, created, removed, or renamed - invalidate relevant caches
					invalidateCachesForFile(event.Name)
				}
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
				// Log the error but continue watching
				// In a production system, you might want to handle this differently
			}
		}
	}()

	return nil
}

// AddFileToWatch adds a file to the watcher and tracks which cache keys depend on it
func AddFileToWatch(filePath string, cacheKey string) error {
	fileWatcher.Lock()
	defer fileWatcher.Unlock()

	if fileWatcher.watcher == nil {
		return nil // watcher not initialized yet
	}

	// Add the directory containing the file to the watcher if not already watched
	_ = fileWatcher.watcher.Add(filepath.Dir(filePath))
	// Note: We ignore errors here as some directories might not be accessible

	// Track which cache keys are affected by this file
	if _, exists := fileWatcher.fileToCacheKeys[filePath]; !exists {
		fileWatcher.fileToCacheKeys[filePath] = make(map[string]bool)
	}
	fileWatcher.fileToCacheKeys[filePath][cacheKey] = true

	return nil
}

// AddDirToWatch adds a directory to the watcher to track new file creation
func AddDirToWatch(dirPath string) error {
	fileWatcher.Lock()
	defer fileWatcher.Unlock()

	if fileWatcher.watcher == nil {
		return nil // watcher not initialized yet
	}

	// Add the directory to the watcher if not already watched
	_ = fileWatcher.watcher.Add(dirPath)
	// Note: We ignore errors here as some directories might not be accessible

	return nil
}

// addFileToWatch is an internal function that calls the exported AddFileToWatch
func addFileToWatch(filePath string, cacheKey string) error {
	return AddFileToWatch(filePath, cacheKey)
}

// invalidateCachesForFile invalidates all cache entries that depend on the specified file
func invalidateCachesForFile(filePath string) {
	fileWatcher.RLock()
	cacheKeys, exists := fileWatcher.fileToCacheKeys[filePath]
	fileWatcher.RUnlock()

	if exists {
		// Invalidate package cache entries
		packageCache.Lock()
		for cacheKey := range cacheKeys {
			delete(packageCache.pkgs, cacheKey)
		}
		packageCache.Unlock()
	}

	// Invalidate file lines cache entry for this specific file
	fileLinesCache.Lock()
	delete(fileLinesCache.data, filePath)
	fileLinesCache.Unlock()

	// Also invalidate any package cache that might include this new file
	// This handles the case where a new file is added to a directory/package
	dir := filepath.Dir(filePath)
	invalidatePackageCachesInDir(dir)
	invalidateFileLinesCachesInDir(dir)
}

// invalidatePackageCachesInDir invalidates all package caches for a specific directory
func invalidatePackageCachesInDir(dir string) {
	packageCache.Lock()
	defer packageCache.Unlock()

	// Find and invalidate all cache entries that might be affected by changes in this directory
	// The cache key includes the directory, so we need to find entries that contain this directory
	for cacheKey, item := range packageCache.pkgs {
		// Check if any of the cached files are in the specified directory
		for file := range item.FileModTime {
			if filepath.Dir(file) == dir {
				delete(packageCache.pkgs, cacheKey)
				break
			}
		}
	}
}

// invalidateFileLinesCachesInDir invalidates all file lines caches for files in a specific directory
func invalidateFileLinesCachesInDir(dir string) {
	fileLinesCache.Lock()
	defer fileLinesCache.Unlock()

	// Find and invalidate all file lines cache entries for files in the specified directory
	for filename := range fileLinesCache.data {
		if filepath.Dir(filename) == dir {
			delete(fileLinesCache.data, filename)
		}
	}
}

// init initializes the cache cleanup processes.
func init() {
	// Clean up package cache entries older than 30 minutes every 10 minutes
	startCacheCleanup(10*time.Minute, 30*time.Minute)

	// Clean up file lines cache entries older than 30 minutes every 10 minutes
	startFileLinesCacheCleanup(10*time.Minute, 30*time.Minute)

	// Initialize and start file watcher
	_ = initFileWatcher() // Error is logged but doesn't stop initialization
}
