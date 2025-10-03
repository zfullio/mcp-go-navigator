package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"go-navigator/internal/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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
		if strings.Contains(p, "testdata") {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("expected testdata package, got %v", out.Packages)
	}
}

func TestListSymbols(t *testing.T) {
	t.Parallel()

	in := tools.ListSymbolsInput{
		Dir:     testDir(),
		Package: "./...",
	}

	_, out, err := tools.ListSymbols(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("ListSymbols error: %v", err)
	}

	kinds := map[string]bool{}
	for _, s := range out.Symbols {
		kinds[s.Kind] = true
	}

	if !kinds["struct"] {
		t.Errorf("expected to find struct, got %+v", out.Symbols)
	}

	if !kinds["func"] {
		t.Errorf("expected to find func, got %+v", out.Symbols)
	}

	if !kinds["interface"] {
		t.Errorf("expected to find interface, got %+v", out.Symbols)
	}

	if !kinds["method"] {
		t.Errorf("expected to find method, got %+v", out.Symbols)
	}
}

func TestFindReferences(t *testing.T) {
	t.Parallel()

	in := tools.FindReferencesInput{
		Dir:   testDir(),
		Ident: "Foo",
	}

	_, out, err := tools.FindReferences(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("FindReferences error: %v", err)
	}

	if len(out.References) == 0 {
		t.Fatalf("expected to find references to Foo, got 0")
	}

	// Проверим, что есть определение типа Foo
	foundType := false
	// Проверим, что есть использование Foo в bar.go
	foundUsage := false

	for _, ref := range out.References {
		if strings.Contains(ref.Snippet, "type Foo struct") {
			foundType = true
		}

		if strings.Contains(ref.Snippet, "UseFoo") || strings.Contains(ref.Snippet, "DoSomething") {
			foundUsage = true
		}
	}

	if !foundType {
		t.Error("expected to find definition of Foo, but not found")
	}

	if !foundUsage {
		t.Error("expected to find usage of Foo, but not found")
	}
}

func TestFindDefinitions(t *testing.T) {
	t.Parallel()

	in := tools.FindDefinitionsInput{
		Dir:   testDir(),
		Ident: "Foo",
	}

	_, out, err := tools.FindDefinitions(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("FindDefinitions error: %v", err)
	}

	if len(out.Definitions) == 0 {
		t.Fatalf("expected definitions of Foo, got 0")
	}

	found := false

	for _, d := range out.Definitions {
		if strings.Contains(d.Snippet, "type Foo struct") {
			found = true
		}
	}

	if !found {
		t.Errorf("expected definition 'type Foo struct', got %+v", out.Definitions)
	}
}

func TestRenameSymbol(t *testing.T) {
	t.Parallel()

	dir := testDir()

	// Создаём копию testdata, чтобы не портить исходники
	tmpDir := filepath.Join(os.TempDir(), "sample_copy")
	_ = os.RemoveAll(tmpDir)

	err := copyDir(dir, tmpDir)
	if err != nil {
		t.Fatalf("copyDir error: %v", err)
	}

	in := tools.RenameSymbolInput{
		Dir:     tmpDir,
		OldName: "Foo",
		NewName: "MyFoo",
	}

	_, out, err := tools.RenameSymbol(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("RenameSymbol error: %v", err)
	}

	if len(out.ChangedFiles) == 0 {
		t.Fatalf("expected changed files, got 0")
	}

	// Проверяем, что Foo реально заменён на MyFoo
	for _, f := range out.ChangedFiles {
		data, _ := os.ReadFile(f)
		if !strings.Contains(string(data), "MyFoo") {
			t.Errorf("expected file %s to contain MyFoo", f)
		}
	}
}

func TestListImports(t *testing.T) {
	in := tools.ListImportsInput{Dir: testDir()}

	_, out, err := tools.ListImports(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("ListImports error: %v", err)
	}

	if len(out.Imports) == 0 {
		t.Fatalf("expected at least 1 import, got 0")
	}

	foundFmt := false
	foundStrings := false
	for _, imp := range out.Imports {
		if imp.Path == "fmt" {
			foundFmt = true
		}
		if imp.Path == "strings" {
			foundStrings = true
		}
	}

	if !foundFmt || !foundStrings {
		t.Errorf("expected to find imports fmt and strings, got %+v", out.Imports)
	}
}

func TestListInterfaces(t *testing.T) {
	in := tools.ListInterfacesInput{Dir: testDir()}

	_, out, err := tools.ListInterfaces(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("ListInterfaces error: %v", err)
	}

	if len(out.Interfaces) == 0 {
		t.Fatalf("expected at least 1 interface, got 0")
	}

	foundStorage := false
	foundSave := false
	foundLoad := false

	for _, iface := range out.Interfaces {
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

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(target, data, info.Mode())
	})
}

func testDir() string {
	_, filename, _, _ := runtime.Caller(0)

	return filepath.Join(filepath.Dir(filename), "testdata", "sample")
}
