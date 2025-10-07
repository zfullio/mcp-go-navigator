package tools_test

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go-navigator/internal/tools"
)

func TestDeadCode(t *testing.T) {
	in := tools.DeadCodeInput{Dir: testDir()}

	_, out, err := tools.DeadCode(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("DeadCode error: %v", err)
	}

	if len(out.Unused) == 0 {
		t.Fatalf("expected unused symbols, got 0")
	}

	// собираем имена
	names := map[string]bool{}
	for _, d := range out.Unused {
		names[d.Name] = true
	}

	// проверяем все dead-символы
	expected := []string{"deadVar", "deadConst", "deadType", "deadFunc"}
	for _, e := range expected {
		if !names[e] {
			t.Errorf("expected to find dead symbol %s, but not found", e)
		}
	}
}

func TestDeadCode_WithPackageFilter(t *testing.T) {
	dir := testDir()
	pkgPath := samplePackagePath(t)

	in := tools.DeadCodeInput{
		Dir:             dir,
		Package:         pkgPath,
		IncludeExported: true,
	}

	_, out, err := tools.DeadCode(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("DeadCode error: %v", err)
	}

	expected := filepath.ToSlash(pkgPath)

	for _, symbol := range out.Unused {
		if filepath.ToSlash(symbol.Package) != expected {
			t.Fatalf("unexpected package %s in results (expected %s)", symbol.Package, pkgPath)
		}
	}

	for pkg := range out.ByPackage {
		if filepath.ToSlash(pkg) != expected {
			t.Fatalf("unexpected package key %s in aggregated results (expected %s)", pkg, pkgPath)
		}
	}
}

func TestDeadCode_WithUnknownPackage(t *testing.T) {
	in := tools.DeadCodeInput{
		Dir:     testDir(),
		Package: "nonexistent/package",
	}

	_, _, err := tools.DeadCode(context.Background(), &mcp.CallToolRequest{}, in)
	if err == nil {
		t.Fatalf("expected error for unknown package, got nil")
	}
}

func TestDeadCodeExtended(t *testing.T) {
	in := tools.DeadCodeInput{
		Dir:             testDir(),
		IncludeExported: true, // Test the extended functionality
	}

	_, out, err := tools.DeadCode(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("DeadCode error: %v", err)
	}

	if len(out.Unused) == 0 {
		t.Fatalf("expected unused symbols, got 0")
	}

	// Check that the extended fields are populated
	if out.TotalCount != len(out.Unused) {
		t.Errorf("expected TotalCount (%d) to equal length of Unused (%d)",
			out.TotalCount, len(out.Unused))
	}

	if len(out.ByPackage) == 0 {
		t.Errorf("expected ByPackage to have entries, got empty map")
	}

	if len(out.ByKind) == 0 {
		t.Errorf("expected ByKind to have entries, got empty map")
	}

	if out.HasMore {
		t.Errorf("did not expect HasMore when no limit is set")
	}

	// Count exported symbols in the results
	exportedCount := 0

	for _, unused := range out.Unused {
		if unused.IsExported {
			exportedCount++
		}

		if unused.Package == "" {
			t.Errorf("expected Package field to be populated for unused symbol %v", unused)
		}
	}

	if out.ExportedCount != exportedCount {
		t.Errorf("expected ExportedCount (%d) to match actual exported count (%d)",
			out.ExportedCount, exportedCount)
	}

	// Test without including exported to ensure filtering works
	in.IncludeExported = false

	_, out2, err := tools.DeadCode(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("DeadCode (IncludeExported=false) error: %v", err)
	}

	// The second run should have fewer or equal unused symbols (since exported ones are filtered)
	exportedCount2 := 0

	for _, unused := range out2.Unused {
		if unused.IsExported {
			exportedCount2++
		}
	}

	if exportedCount2 > 0 {
		t.Errorf("expected no exported symbols when IncludeExported=false, but found %d", exportedCount2)
	}

	if out2.HasMore {
		t.Errorf("did not expect HasMore when no limit is set (second run)")
	}
}

func TestDeadCode_WithInvalidDir(t *testing.T) {
	in := tools.DeadCodeInput{Dir: "/nonexistent/directory"}

	_, _, err := tools.DeadCode(context.Background(), &mcp.CallToolRequest{}, in)
	if err == nil {
		t.Fatalf("expected error for non-existent directory, got nil")
	}
}

func TestDeadCodeWithMethods(t *testing.T) {
	in := tools.DeadCodeInput{Dir: testDir()}

	_, out, err := tools.DeadCode(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("DeadCode error: %v", err)
	}

	names := map[string]bool{}
	for _, d := range out.Unused {
		names[d.Name] = true
	}

	// проверяем, что deadHelper найден
	if !names["deadHelper"] {
		t.Errorf("expected to find unused method 'deadHelper', but not found")
	}

	// DoSomething используется → не должен попасть в deadCode
	if names["DoSomething"] {
		t.Errorf("did not expect 'DoSomething' to be marked as unused")
	}
}

func TestDeadCode_AllKinds(t *testing.T) {
	in := tools.DeadCodeInput{Dir: testDir()}

	_, out, err := tools.DeadCode(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("DeadCode error: %v", err)
	}

	names := map[string]bool{}
	for _, d := range out.Unused {
		names[d.Name] = true
	}

	// Проверяем приватный метод
	if !names["deadHelper"] {
		t.Errorf("expected to find unused method 'deadHelper', but not found")
	}

	// Проверяем переменную
	if !names["unusedVar"] {
		t.Errorf("expected to find unused variable 'unusedVar', but not found")
	}

	// Проверяем константу
	if !names["unusedConst"] {
		t.Errorf("expected to find unused constant 'unusedConst', but not found")
	}

	// Проверяем тип
	if !names["unusedType"] {
		t.Errorf("expected to find unused type 'unusedType', but not found")
	}

	// Проверяем, что DoSomething (используется) не попал
	if names["DoSomething"] {
		t.Errorf("did not expect 'DoSomething' to be marked as unused")
	}
}

func TestDeadCode_WithLimit(t *testing.T) {
	in := tools.DeadCodeInput{Dir: testDir(), Limit: 2}

	_, out, err := tools.DeadCode(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("DeadCode error: %v", err)
	}

	if len(out.Unused) != 2 {
		t.Fatalf("expected exactly 2 unused symbols due to limit, got %d", len(out.Unused))
	}

	if !out.HasMore {
		t.Fatalf("expected HasMore to be true when limit is applied and more results exist")
	}

	if out.TotalCount <= len(out.Unused) {
		t.Fatalf("expected TotalCount (%d) to exceed returned unused symbols (%d)", out.TotalCount, len(out.Unused))
	}

	if len(out.ByKind) == 0 {
		t.Fatalf("expected ByKind to be populated")
	}
}

func TestAnalyzeDependencies(t *testing.T) {
	in := tools.AnalyzeDependenciesInput{Dir: testDir()}

	_, out, err := tools.AnalyzeDependencies(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("AnalyzeDependencies error: %v", err)
	}

	if len(out.Dependencies) == 0 {
		t.Fatalf("expected at least 1 dependency, got 0")
	}

	// Check that we have basic dependency information
	hasImports := false

	for _, dep := range out.Dependencies {
		if len(dep.Imports) > 0 {
			hasImports = true

			break
		}
	}

	if !hasImports {
		t.Errorf("expected at least one package with imports, got %+v", out.Dependencies)
	}

	// Check for dependency cycles (there shouldn't be any in sample data)
	if len(out.Cycles) > 0 {
		t.Logf("Found dependency cycles: %+v", out.Cycles)
	}
}

func TestAnalyzeDependencies_WithPackageFilter(t *testing.T) {
	dir := projectRoot()
	pkgPath := toolsPackagePath(t, dir)

	in := tools.AnalyzeDependenciesInput{
		Dir:     dir,
		Package: pkgPath,
	}

	_, out, err := tools.AnalyzeDependencies(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("AnalyzeDependencies error: %v", err)
	}

	if len(out.Dependencies) == 0 {
		t.Fatalf("expected dependencies for filtered package, got 0")
	}

	for _, dep := range out.Dependencies {
		if dep.Package != pkgPath {
			t.Fatalf("expected only package %s, got %s", pkgPath, dep.Package)
		}
	}
}

func TestAnalyzeDependencies_WithUnknownPackage(t *testing.T) {
	dir := projectRoot()
	in := tools.AnalyzeDependenciesInput{
		Dir:     dir,
		Package: "nonexistent/package",
	}

	_, _, err := tools.AnalyzeDependencies(context.Background(), &mcp.CallToolRequest{}, in)
	if err == nil {
		t.Fatalf("expected error for unknown package, got nil")
	}
}

func TestAnalyzeDependencies_WithInvalidDir(t *testing.T) {
	in := tools.AnalyzeDependenciesInput{Dir: "/nonexistent/directory"}

	_, _, err := tools.AnalyzeDependencies(context.Background(), &mcp.CallToolRequest{}, in)
	if err == nil {
		t.Fatalf("expected error for non-existent directory, got nil")
	}
}

func TestAnalyzeComplexity(t *testing.T) {
	in := tools.AnalyzeComplexityInput{Dir: testDir()}

	_, out, err := tools.AnalyzeComplexity(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("AnalyzeComplexity error: %v", err)
	}

	if len(out.Functions) == 0 {
		t.Fatalf("expected at least 1 function group, got 0")
	}

	funcs := map[string]tools.FunctionComplexityInfo{}

	for _, group := range out.Functions {
		for _, fn := range group.Functions {
			funcs[fn.Name] = fn
		}
	}

	if f, ok := funcs["Simple"]; !ok {
		t.Errorf("expected function Simple, got %+v", funcs)
	} else if f.Cyclomatic != 1 {
		t.Errorf("expected Simple cyclomatic=1, got %d", f.Cyclomatic)
	}

	if f, ok := funcs["WithIf"]; !ok {
		t.Errorf("expected function WithIf, got %+v", funcs)
	} else {
		if f.Cyclomatic < 2 {
			t.Errorf("expected WithIf cyclomatic>=2, got %d", f.Cyclomatic)
		}

		if f.Nesting < 1 {
			t.Errorf("expected WithIf nesting>=1, got %d", f.Nesting)
		}
	}

	if f, ok := funcs["WithLoopAndSwitch"]; !ok {
		t.Errorf("expected function WithLoopAndSwitch, got %+v", funcs)
	} else {
		if f.Cyclomatic < 3 {
			t.Errorf("expected WithLoopAndSwitch cyclomatic>=3, got %d", f.Cyclomatic)
		}

		if f.Nesting < 2 {
			t.Errorf("expected WithLoopAndSwitch nesting>=2, got %d", f.Nesting)
		}
	}
}

func TestAnalyzeComplexity_WithPackageFilter(t *testing.T) {
	dir := projectRoot()
	pkgPath := toolsPackagePath(t, dir)

	in := tools.AnalyzeComplexityInput{
		Dir:     dir,
		Package: pkgPath,
	}

	_, out, err := tools.AnalyzeComplexity(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("AnalyzeComplexity error: %v", err)
	}

	if len(out.Functions) == 0 {
		t.Fatalf("expected function groups for filtered package, got 0")
	}

	if normalized := filepath.ToSlash(pkgPath); !strings.HasSuffix(normalized, "/internal/tools") && normalized != "internal/tools" {
		t.Fatalf("unexpected package path for internal/tools: %s", pkgPath)
	}

	for _, group := range out.Functions {
		file := filepath.ToSlash(group.File)
		if !strings.HasPrefix(file, "internal/tools/") {
			t.Fatalf("unexpected file %s for filtered package", group.File)
		}
	}
}

func TestAnalyzeComplexity_WithUnknownPackage(t *testing.T) {
	dir := projectRoot()
	in := tools.AnalyzeComplexityInput{
		Dir:     dir,
		Package: "nonexistent/package",
	}

	_, _, err := tools.AnalyzeComplexity(context.Background(), &mcp.CallToolRequest{}, in)
	if err == nil {
		t.Fatalf("expected error for unknown package, got nil")
	}
}

func TestAnalyzeComplexity_WithInvalidDir(t *testing.T) {
	in := tools.AnalyzeComplexityInput{Dir: "/nonexistent/directory"}

	_, _, err := tools.AnalyzeComplexity(context.Background(), &mcp.CallToolRequest{}, in)
	if err == nil {
		t.Fatalf("expected error for non-existent directory, got nil")
	}
}

func TestMetricsSummary(t *testing.T) {
	in := tools.MetricsSummaryInput{Dir: testDir()}

	_, out, err := tools.MetricsSummary(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("MetricsSummary error: %v", err)
	}

	// Check that we have reasonable metrics
	if out.PackageCount <= 0 {
		t.Errorf("expected at least 1 package, got %d", out.PackageCount)
	}

	if out.StructCount < 0 {
		t.Errorf("expected non-negative struct count, got %d", out.StructCount)
	}

	if out.InterfaceCount < 0 {
		t.Errorf("expected non-negative interface count, got %d", out.InterfaceCount)
	}

	if out.FunctionCount < 0 {
		t.Errorf("expected non-negative function count, got %d", out.FunctionCount)
	}

	if out.LineCount <= 0 {
		t.Errorf("expected positive line count, got %d", out.LineCount)
	}

	if out.FileCount <= 0 {
		t.Errorf("expected positive file count, got %d", out.FileCount)
	}

	if out.AverageCyclomatic < 0 {
		t.Errorf("expected non-negative average cyclomatic complexity, got %f", out.AverageCyclomatic)
	}

	if out.DeadCodeCount < 0 {
		t.Errorf("expected non-negative dead code count, got %d", out.DeadCodeCount)
	}

	if out.ExportedUnusedCount < 0 {
		t.Errorf("expected non-negative exported unused count, got %d", out.ExportedUnusedCount)
	}

	if out.ExportedUnusedCount > out.DeadCodeCount {
		t.Errorf("expected exported unused count (%d) to be <= total dead code count (%d)",
			out.ExportedUnusedCount, out.DeadCodeCount)
	}

	// Basic metrics should be computed
}

func TestMetricsSummary_WithPackageFilter(t *testing.T) {
	dir := projectRoot()
	pkgPath := toolsPackagePath(t, dir)

	in := tools.MetricsSummaryInput{
		Dir:     dir,
		Package: pkgPath,
	}

	_, out, err := tools.MetricsSummary(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("MetricsSummary error: %v", err)
	}

	if out.PackageCount != 1 {
		t.Fatalf("expected package count 1, got %d", out.PackageCount)
	}

	if out.FunctionCount < 0 {
		t.Fatalf("expected non-negative function count, got %d", out.FunctionCount)
	}

	if out.LineCount < 0 {
		t.Fatalf("expected non-negative line count, got %d", out.LineCount)
	}

	if out.DeadCodeCount < 0 || out.ExportedUnusedCount < 0 {
		t.Fatalf("expected non-negative dead code counts, got %d/%d", out.DeadCodeCount, out.ExportedUnusedCount)
	}
}

func TestMetricsSummary_WithUnknownPackage(t *testing.T) {
	in := tools.MetricsSummaryInput{
		Dir:     projectRoot(),
		Package: "nonexistent/package",
	}

	_, _, err := tools.MetricsSummary(context.Background(), &mcp.CallToolRequest{}, in)
	if err == nil {
		t.Fatalf("expected error for unknown package, got nil")
	}
}

func TestMetricsSummary_WithInvalidDir(t *testing.T) {
	in := tools.MetricsSummaryInput{Dir: "/nonexistent/directory"}

	_, _, err := tools.MetricsSummary(context.Background(), &mcp.CallToolRequest{}, in)
	if err == nil {
		t.Fatalf("expected error for non-existent directory, got nil")
	}
}

func projectRoot() string {
	return filepath.Clean(filepath.Join(testDir(), "..", "..", "..", ".."))
}

func toolsPackagePath(t *testing.T, dir string) string {
	return packagePathBySuffix(t, dir, "internal/tools")
}

func samplePackagePath(t *testing.T) string {
	return packagePathBySuffix(t, testDir(), "sample")
}

func packagePathBySuffix(t *testing.T, dir, suffix string) string {
	t.Helper()

	_, out, err := tools.ListPackages(context.Background(), &mcp.CallToolRequest{}, tools.ListPackagesInput{Dir: dir})
	if err != nil {
		t.Fatalf("ListPackages error: %v", err)
	}

	target := strings.TrimPrefix(filepath.ToSlash(suffix), "/")

	for _, pkgPath := range out.Packages {
		normalized := strings.TrimPrefix(filepath.ToSlash(pkgPath), "/")
		if normalized == target || strings.HasSuffix(normalized, "/"+target) {
			return pkgPath
		}
	}

	t.Fatalf("package with suffix %q not found in %v", suffix, out.Packages)

	return ""
}

func BenchmarkDeadCode(b *testing.B) {
	in := tools.DeadCodeInput{
		Dir: benchDir(), // твоя тестовая директория
	}

	for range b.N {
		_, _, err := tools.DeadCode(context.Background(), &mcp.CallToolRequest{}, in)
		if err != nil {
			b.Fatalf("DeadCode error: %v", err)
		}
	}
}

func BenchmarkAnalyzeComplexity(b *testing.B) {
	in := tools.AnalyzeComplexityInput{
		Dir: benchDir(), // твоя тестовая директория
	}

	for range b.N {
		_, _, err := tools.AnalyzeComplexity(context.Background(), &mcp.CallToolRequest{}, in)
		if err != nil {
			b.Fatalf("AnalyzeComplexity error: %v", err)
		}
	}
}

func BenchmarkComplexityVisitor(b *testing.B) {
	// Take one specific file to measure only the visitor
	dir := benchDir()
	fset := token.NewFileSet()
	file := filepath.Join(dir, "complex.go") // возьми тестовый файл с функциями

	node, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		b.Fatalf("parse error: %v", err)
	}

	// ищем первую функцию
	var fn *ast.FuncDecl
	ast.Inspect(node, func(n ast.Node) bool {
		if f, ok := n.(*ast.FuncDecl); ok {
			fn = f

			return false
		}

		return true
	})

	if fn == nil {
		b.Fatal("no function found in example.go")
	}

	b.ResetTimer()

	for range b.N {
		visitor := &tools.ComplexityVisitor{
			Ctx:        context.Background(),
			Fset:       fset,
			Nesting:    0,
			MaxNesting: 0,
			Cyclomatic: 1,
		}
		ast.Walk(visitor, fn.Body)
		_ = visitor.MaxNesting
		_ = visitor.Cyclomatic
	}
}
