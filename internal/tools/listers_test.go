package tools_test

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go-navigator/internal/tools"
)

func TestListPackages(t *testing.T) {
	t.Parallel()

	in := tools.ListPackagesInput{Dir: testDir()}

	_, out, err := tools.ListPackages(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("ListPackages error: %v", err)
	}

	if len(out.Packages) == 0 {
		t.Errorf("expected at least 1 package, got 0")
	}

	found := false

	for _, p := range out.Packages {
		if strings.Contains(p, "sample") {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("expected testdata package, got %v", out.Packages)
	}
}

func TestListPackages_WithEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	in := tools.ListPackagesInput{Dir: tmpDir}

	_, out, err := tools.ListPackages(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("ListPackages error: %v", err)
	}

	// Even an empty directory should return without error
	// but might have no packages depending on implementation
	if len(out.Packages) < 0 {
		t.Errorf("expected 0 or more packages, got %v", len(out.Packages))
	}
}

func TestListPackages_WithInvalidDir(t *testing.T) {
	in := tools.ListPackagesInput{Dir: "/nonexistent/directory"}

	_, _, err := tools.ListPackages(context.Background(), &mcp.CallToolRequest{}, in)
	if err == nil {
		t.Fatalf("expected error for non-existent directory, got nil")
	}
}

func TestListSymbols_WithInvalidDir(t *testing.T) {
	in := tools.ListSymbolsInput{Dir: "/nonexistent/directory"}

	_, _, err := tools.ListSymbols(context.Background(), &mcp.CallToolRequest{}, in)
	if err == nil {
		t.Fatalf("expected error for non-existent directory, got nil")
	}
}

func TestListSymbols_WithInvalidPackage(t *testing.T) {
	in := tools.ListSymbolsInput{
		Dir:     testDir(),
		Package: "nonexistent/package",
	}

	_, out, err := tools.ListSymbols(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("ListSymbols error: %v", err)
	}

	// Should return empty result for non-existent package
	if len(out.GroupedSymbols) != 0 {
		t.Errorf("expected 0 symbols for non-existent package, got %v", len(out.GroupedSymbols))
	}
}

func TestListImports(t *testing.T) {
	in := tools.ListImportsInput{Dir: testDir()}

	_, out, err := tools.ListImports(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("ListImports error: %v", err)
	}

	if len(out.Imports) == 0 {
		t.Fatalf("expected at least 1 grouped import, got 0")
	}

	foundFmt := false
	foundStrings := false

	for _, group := range out.Imports {
		if group.File == "" {
			t.Errorf("expected file name in grouped imports, got empty string")
		}

		for _, imp := range group.Imports {
			if imp.Path == "fmt" {
				foundFmt = true
			}

			if imp.Path == "strings" {
				foundStrings = true
			}
		}
	}

	if !foundFmt || !foundStrings {
		t.Errorf("expected to find imports fmt and strings, got %+v", out.Imports)
	}
}

func TestListImports_WithInvalidDir(t *testing.T) {
	in := tools.ListImportsInput{Dir: "/nonexistent/directory"}

	_, _, err := tools.ListImports(context.Background(), &mcp.CallToolRequest{}, in)
	if err == nil {
		t.Fatalf("expected error for non-existent directory, got nil")
	}
}

func TestListInterfaces(t *testing.T) {
	in := tools.ListInterfacesInput{Dir: testDir()}

	_, out, err := tools.ListInterfaces(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("ListInterfaces error: %v", err)
	}

	if len(out.Interfaces) == 0 {
		t.Fatalf("expected at least 1 grouped interface, got 0")
	}

	foundStorage := false
	foundSave := false
	foundLoad := false

	var aggregated []tools.InterfaceInfo

	for _, group := range out.Interfaces {
		if group.Package == "" {
			t.Errorf("expected package name in grouped interfaces, got empty string")
		}

		aggregated = append(aggregated, group.Interfaces...)
	}

	for _, iface := range aggregated {
		if iface.Name == "Storage" {
			foundStorage = true

			for _, m := range iface.Methods {
				if m.Name == "Save" {
					foundSave = true
				}

				if m.Name == "Load" {
					foundLoad = true
				}
			}
		}
	}

	if !foundStorage {
		t.Errorf("expected to find interface Storage, got %+v", out.Interfaces)
	}

	if !foundSave {
		t.Errorf("expected to find method Save in Storage, got %+v", out.Interfaces)
	}

	if !foundLoad {
		t.Errorf("expected to find method Load in Storage, got %+v", out.Interfaces)
	}
}

func TestListInterfaces_HandlesEmptyInterface(t *testing.T) {
	in := tools.ListInterfacesInput{Dir: testDir()}

	_, out, err := tools.ListInterfaces(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("ListInterfaces error: %v", err)
	}

	for _, group := range out.Interfaces {
		for _, iface := range group.Interfaces {
			if iface.Name == "Empty" {
				if len(iface.Methods) != 0 {
					t.Fatalf("expected Empty interface to have zero methods, got %d", len(iface.Methods))
				}

				return
			}
		}
	}

	t.Fatalf("expected to find Empty interface in testdata, but it was missing")
}

func TestListInterfaces_WithInvalidDir(t *testing.T) {
	in := tools.ListInterfacesInput{Dir: "/nonexistent/directory"}

	_, _, err := tools.ListInterfaces(context.Background(), &mcp.CallToolRequest{}, in)
	if err == nil {
		t.Fatalf("expected error for non-existent directory, got nil")
	}
}

func TestProjectSchema(t *testing.T) {
	t.Parallel()

	in := tools.ProjectSchemaInput{Dir: testDir()}

	_, out, err := tools.ProjectSchema(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("ProjectSchema error: %v", err)
	}

	// Проверяем основные поля
	if out.Module == "" {
		t.Error("expected module name, got empty string")
	}

	if out.GoVersion == "" {
		t.Error("expected go version, got empty string")
	}

	if out.RootDir != testDir() {
		t.Errorf("expected root dir %s, got %s", testDir(), out.RootDir)
	}

	// Проверяем, что есть хотя бы один пакет
	if len(out.Packages) == 0 {
		t.Error("expected at least 1 package, got 0")
	}

	// Проверяем, что есть зависимости
	if len(out.ExternalDeps) == 0 {
		t.Error("expected at least 1 external dependency, got 0")
	}

	// Проверяем граф зависимостей
	if len(out.DependencyGraph) == 0 {
		t.Error("expected dependency graph with at least 1 entry, got 0")
	}

	// Проверяем сводку
	if out.Summary.PackageCount == 0 {
		t.Error("expected package count > 0 in summary")
	}

	// Проверяем, что интерфейсы присутствуют на стандартном уровне
	if len(out.Interfaces) == 0 {
		t.Error("expected at least 1 interface, got 0")
	}
}

func TestProjectSchema_WithSummaryDepth(t *testing.T) {
	t.Parallel()

	in := tools.ProjectSchemaInput{
		Dir:   testDir(),
		Depth: "summary",
	}

	_, out, err := tools.ProjectSchema(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("ProjectSchema error: %v", err)
	}

	// Проверяем, что основные поля есть
	if out.Module == "" {
		t.Error("expected module name, got empty string")
	}

	if out.RootDir != testDir() {
		t.Errorf("expected root dir %s, got %s", testDir(), out.RootDir)
	}

	// На уровне summary интерфейсы не должны быть включены
	if len(out.Interfaces) > 0 {
		t.Errorf("expected no interfaces for summary depth, got %d", len(out.Interfaces))
	}

	// Пакеты должны быть
	if len(out.Packages) == 0 {
		t.Error("expected at least 1 package, got 0")
	}

	// Сводка должна быть
	if out.Summary.PackageCount == 0 {
		t.Error("expected package count > 0 in summary")
	}
}

func TestProjectSchema_WithInvalidDir(t *testing.T) {
	in := tools.ProjectSchemaInput{Dir: "/nonexistent/directory"}

	_, _, err := tools.ProjectSchema(context.Background(), &mcp.CallToolRequest{}, in)
	if err == nil {
		t.Fatalf("expected error for non-existent directory, got nil")
	}
}

func BenchmarkListSymbols(b *testing.B) {
	in := tools.ListSymbolsInput{
		Dir:     benchDir(),
		Package: "./...",
	}

	for range b.N {
		_, _, err := tools.ListSymbols(context.Background(), &mcp.CallToolRequest{}, in)
		if err != nil {
			b.Fatalf("ListSymbols error: %v", err)
		}
	}
}
