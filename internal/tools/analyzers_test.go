package tools_test

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"go-navigator/internal/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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

func TestMetricsSummary_WithInvalidDir(t *testing.T) {
	in := tools.MetricsSummaryInput{Dir: "/nonexistent/directory"}

	_, _, err := tools.MetricsSummary(context.Background(), &mcp.CallToolRequest{}, in)
	if err == nil {
		t.Fatalf("expected error for non-existent directory, got nil")
	}
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
