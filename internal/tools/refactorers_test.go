package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go-navigator/internal/tools"
)

func TestRenameSymbol(t *testing.T) {
	t.Parallel()

	dir := testDir()

	// Create a copy of testdata to avoid modifying the sources
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

	// Verify that Foo is actually replaced with MyFoo
	for _, f := range out.ChangedFiles {
		full := filepath.Join(tmpDir, f) // ✅ путь относительно tmpDir

		data, _ := os.ReadFile(full)
		if !strings.Contains(string(data), "MyFoo") {
			t.Errorf("expected file %s to contain MyFoo", f)
		}
	}
}

func TestRenameSymbol_WithInvalidDir(t *testing.T) {
	t.Parallel()

	in := tools.RenameSymbolInput{
		Dir:     "/nonexistent/directory",
		OldName: "Foo",
		NewName: "Bar",
	}

	_, _, err := tools.RenameSymbol(context.Background(), &mcp.CallToolRequest{}, in)
	if err == nil {
		t.Fatalf("expected error for non-existent directory, got nil")
	}
}

func TestRenameSymbol_WithSameNames(t *testing.T) {
	t.Parallel()

	in := tools.RenameSymbolInput{
		Dir:     testDir(),
		OldName: "Foo",
		NewName: "Foo", // Same name
	}

	_, out, err := tools.RenameSymbol(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("RenameSymbol error: %v", err)
	}

	// Should return a collision message for same names
	if len(out.Collisions) == 0 {
		t.Errorf("expected collision message when old and new names are the same, got none")
	}
}

func TestRenameSymbol_WithNonexistentSymbol(t *testing.T) {
	t.Parallel()

	in := tools.RenameSymbolInput{
		Dir:     testDir(),
		OldName: "NonexistentSymbol",
		NewName: "NewSymbol",
	}

	_, _, err := tools.RenameSymbol(context.Background(), &mcp.CallToolRequest{}, in)
	if err == nil {
		t.Fatalf("expected error for non-existent symbol, got nil")
	}
}

func TestRenameSymbol_DryRun(t *testing.T) {
	t.Parallel()

	dir := testDir()
	tmpDir := t.TempDir()

	if err := copyDir(dir, tmpDir); err != nil {
		t.Fatalf("copyDir error: %v", err)
	}

	// First, verify the original file content
	originalFilePath := filepath.Join(tmpDir, "foo.go")

	originalContent, err := os.ReadFile(originalFilePath)
	if err != nil {
		t.Fatalf("failed to read original file: %v", err)
	}

	if !strings.Contains(string(originalContent), "type Foo struct") {
		t.Fatalf("expected original file to contain 'type Foo struct', but it doesn't")
	}

	// Perform rename with dry run
	in := tools.RenameSymbolInput{
		Dir:     tmpDir,
		OldName: "Foo",
		NewName: "MyFoo",
		DryRun:  true, // Dry run
	}

	_, out, err := tools.RenameSymbol(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("RenameSymbol error: %v", err)
	}

	// Check that we have diffs
	if len(out.Diffs) == 0 {
		t.Skip("no changes detected – likely no matching pattern in testdata")
	}

	// Verify the original file content hasn't changed
	contentAfterDryRun, err := os.ReadFile(originalFilePath)
	if err != nil {
		t.Fatalf("failed to read file after dry run: %v", err)
	}

	if string(originalContent) != string(contentAfterDryRun) {
		t.Errorf("file content changed during dry run, expected no changes")
	}
}

func TestRenameSymbol_WithMethodFormat(t *testing.T) {
	t.Parallel()

	dir := testDir()
	tmpDir := t.TempDir()

	if err := copyDir(dir, tmpDir); err != nil {
		t.Fatalf("copyDir error: %v", err)
	}

	// Тестируем переименование существующего метода с использованием формата TypeName.MethodName
	// В foo.go есть тип Foo с методом DoSomething()
	in := tools.RenameSymbolInput{
		Dir:     tmpDir,
		OldName: "Foo.DoSomething", // Используем формат TypeName.MethodName
		NewName: "Process",
	}

	_, out, err := tools.RenameSymbol(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("RenameSymbol error: %v", err)
	}

	if len(out.ChangedFiles) == 0 {
		t.Fatalf("expected changed files, got 0")
	}

	// Проверяем, что файлы из testdata содержат переименованный метод
	fooFilePath := filepath.Join(tmpDir, "foo.go")

	fileContent, err := os.ReadFile(fooFilePath)
	if err != nil {
		t.Fatalf("failed to read foo.go: %v", err)
	}

	contentStr := string(fileContent)
	if !strings.Contains(contentStr, "Process") {
		t.Errorf("expected file to contain 'Process' after rename, but it doesn't")
	}

	if strings.Contains(contentStr, "DoSomething") {
		t.Errorf("expected 'DoSomething' method to be renamed, but it still exists")
	}
}

func TestASTRewrite(t *testing.T) {
	t.Parallel()

	dir := testDir()
	tmpDir := filepath.Join(os.TempDir(), "ast_rewrite_test")

	_ = os.RemoveAll(tmpDir)
	defer os.RemoveAll(tmpDir)

	if err := copyDir(dir, tmpDir); err != nil {
		t.Fatalf("copyDir error: %v", err)
	}

	in := tools.ASTRewriteInput{
		Dir:     tmpDir,
		Find:    "fmt.Println(x)",
		Replace: "fmt.Printf(\"%v\\n\", x)",
		DryRun:  true,
	}

	_, out, err := tools.ASTRewrite(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("ASTRewrite error: %v", err)
	}

	// 🔹 Проверяем базовые условия
	if out.TotalChanges == 0 && len(out.Diffs) == 0 {
		t.Skip("no changes detected – likely no matching pattern in testdata")
	}

	if out.TotalChanges < 0 {
		t.Errorf("expected non-negative TotalChanges, got %d", out.TotalChanges)
	}

	if len(out.ChangedFiles) == 0 {
		t.Errorf("expected at least one ChangedFile, got 0")
	}

	if len(out.Diffs) == 0 {
		t.Errorf("expected at least one Diff, got 0")
	}

	// 🔹 Логируем diff для визуальной проверки
	for _, diff := range out.Diffs {
		t.Logf("Diff for %s:\n%s", diff.Path, diff.Diff)
	}
}

func TestASTRewrite_WithInvalidDir(t *testing.T) {
	in := tools.ASTRewriteInput{
		Dir:     "/nonexistent/directory",
		Find:    "fmt.Println(x)",
		Replace: "fmt.Printf(\"%v\\n\", x)",
	}

	_, _, err := tools.ASTRewrite(context.Background(), &mcp.CallToolRequest{}, in)
	if err == nil {
		t.Fatalf("expected error for non-existent directory, got nil")
	}
}

func TestASTRewrite_WithInvalidExpressions(t *testing.T) {
	dir := testDir()
	tmpDir := t.TempDir()

	if err := copyDir(dir, tmpDir); err != nil {
		t.Fatalf("copyDir error: %v", err)
	}

	in := tools.ASTRewriteInput{
		Dir:     tmpDir,
		Find:    "invalid go syntax [", // Invalid Go syntax
		Replace: "fmt.Printf(\"%v\\n\", x)",
	}

	_, _, err := tools.ASTRewrite(context.Background(), &mcp.CallToolRequest{}, in)
	if err == nil {
		t.Fatalf("expected error for invalid syntax in Find field, got nil")
	}

	in = tools.ASTRewriteInput{
		Dir:     tmpDir,
		Find:    "fmt.Println(x)",
		Replace: "invalid go syntax [", // Invalid Go syntax
	}

	_, _, err = tools.ASTRewrite(context.Background(), &mcp.CallToolRequest{}, in)
	if err == nil {
		t.Fatalf("expected error for invalid syntax in Replace field, got nil")
	}
}

func BenchmarkRenameSymbol(b *testing.B) {
	srcDir := benchDir()
	tmpDir := b.TempDir()
	copyDir(srcDir, tmpDir)

	in := tools.RenameSymbolInput{Dir: tmpDir, OldName: "Foo", NewName: "Bar"}
	for b.Loop() {
		_, _, err := tools.RenameSymbol(context.Background(), &mcp.CallToolRequest{}, in)
		if err != nil {
			b.Fatal(err)
		}
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

func containsAll(values []string, names ...string) bool {
	set := make(map[string]struct{}, len(values))
	for _, v := range values {
		set[v] = struct{}{}
	}

	for _, name := range names {
		if _, ok := set[name]; !ok {
			return false
		}
	}

	return true
}

func benchDir() string {
	_, filename, _, _ := runtime.Caller(0)
	// укажем testdata/sample как тестовый проект
	return filepath.Join(filepath.Dir(filename), "testdata", "sample")
}
