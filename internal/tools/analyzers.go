package tools

import (
	"context"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/go/packages"
)

// DeadCode finds unused functions, variables, constants and types in a Go project.
//
// Parameters:
//   - ctx: execution context
//   - req: MCP tool request
//   - input: input data specifying the directory and flag for including exported symbols
//
// Returns:
//   - MCP tool call result
//   - unused code search result
//   - error if an error occurred while loading packages
func DeadCode(ctx context.Context, req *mcp.CallToolRequest, input DeadCodeInput) (
	*mcp.CallToolResult,
	DeadCodeOutput,
	error,
) {
	start := logStart("DeadCode", map[string]string{"dir": input.Dir})
	out := DeadCodeOutput{
		Unused:    []DeadSymbol{},
		ByPackage: make(map[string]int),
	}

	defer func() { logEnd("DeadCode", start, len(out.Unused)) }()

	mode := packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedCompiledGoFiles | packages.NeedName

	pkgs, err := loadPackagesWithCache(ctx, input.Dir, mode)
	if err != nil {
		logError("DeadCode", err, "failed to load packages")

		return fail(out, err)
	}

	exportedCount := 0

	for _, pkg := range pkgs {
		used := make(map[types.Object]struct{}, len(pkg.TypesInfo.Uses))
		for _, obj := range pkg.TypesInfo.Uses {
			if obj != nil {
				used[obj] = struct{}{}
			}
		}

		for ident, obj := range pkg.TypesInfo.Defs {
			if obj == nil || !isDeadCandidate(ident, obj) {
				continue
			}

			if _, ok := used[obj]; ok {
				continue
			}

			// Check if the symbol is exported
			isExported := ast.IsExported(ident.Name)

			// Skip exported symbols if not requested
			if isExported && !input.IncludeExported {
				continue
			}

			pos := pkg.Fset.Position(ident.Pos())
			rel, _ := filepath.Rel(input.Dir, pos.Filename)

			symbol := DeadSymbol{
				Name:       ident.Name,
				Kind:       objStringKind(obj),
				File:       rel,
				Line:       pos.Line,
				IsExported: isExported,
				Package:    pkg.PkgPath,
			}

			out.Unused = append(out.Unused, symbol)

			if isExported {
				exportedCount++
			}

			// Update package count
			out.ByPackage[pkg.PkgPath]++
		}
	}

	out.TotalCount = len(out.Unused)
	out.ExportedCount = exportedCount

	return nil, out, nil
}

// AnalyzeDependencies builds a graph of dependencies between internal packages (imports, cycles, fan-in/fan-out).
//
// Parameters:
//   - ctx: execution context
//   - req: MCP tool request
//   - input: input data specifying the directory for analysis
//
// Returns:
//   - MCP tool call result
//   - package dependency analysis result
//   - error if an error occurred while loading packages
func AnalyzeDependencies(ctx context.Context, req *mcp.CallToolRequest, input AnalyzeDependenciesInput) (
	*mcp.CallToolResult,
	AnalyzeDependenciesOutput,
	error,
) {
	start := logStart("AnalyzeDependencies", map[string]string{"dir": input.Dir})
	out := AnalyzeDependenciesOutput{
		Dependencies: []PackageDependency{},
		Cycles:       [][]string{},
	}

	defer func() { logEnd("AnalyzeDependencies", start, len(out.Dependencies)) }()

	mode := packages.NeedName | packages.NeedImports | packages.NeedCompiledGoFiles

	pkgs, err := loadPackagesWithCache(ctx, input.Dir, mode)
	if err != nil {
		logError("AnalyzeDependencies", err, "failed to load packages")

		return fail(out, err)
	}

	// Build dependency graph
	depGraph := make(map[string][]string)
	pkgMap := make(map[string]*packages.Package)

	for _, pkg := range pkgs {
		pkgMap[pkg.PkgPath] = pkg

		imports := []string{}
		for impPath := range pkg.Imports {
			imports = append(imports, impPath)
			depGraph[pkg.PkgPath] = append(depGraph[pkg.PkgPath], impPath)
		}
	}

	// Calculate fan-in for each package
	fanIn := make(map[string]int)

	for _, pkg := range pkgs {
		for _, impPath := range depGraph[pkg.PkgPath] {
			fanIn[impPath]++
		}
	}

	// Create dependency entries
	for _, pkg := range pkgs {
		imports := []string{}
		for impPath := range pkg.Imports {
			imports = append(imports, impPath)
		}

		fanOut := len(imports)
		fanInCount := fanIn[pkg.PkgPath]

		out.Dependencies = append(out.Dependencies, PackageDependency{
			Package: pkg.PkgPath,
			Imports: imports,
			FanIn:   fanInCount,
			FanOut:  fanOut,
		})
	}

	// Detect cycles using DFS
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	path := []string{}

	var dfs func(string) bool

	dfs = func(pkgPath string) bool {
		if !visited[pkgPath] {
			visited[pkgPath] = true
			recStack[pkgPath] = true
			path = append(path, pkgPath)

			deps, exists := depGraph[pkgPath]
			if exists {
				for _, dep := range deps {
					if !visited[dep] && dfs(dep) {
						return true
					} else if recStack[dep] {
						// Found cycle
						cycleStart := 0

						for i, p := range path {
							if p == dep {
								cycleStart = i

								break
							}
						}

						cycle := path[cycleStart:]
						out.Cycles = append(out.Cycles, cycle)

						return true
					}
				}
			}
		}

		if len(path) > 0 {
			path = path[:len(path)-1]
		}

		recStack[pkgPath] = false

		return false
	}

	for pkgPath := range pkgMap {
		if !visited[pkgPath] {
			dfs(pkgPath)
		}
	}

	return nil, out, nil
}

// AnalyzeComplexity analyzes function metrics: lines of code, nesting depth, and cyclomatic complexity.
//
// Parameters:
//   - ctx: execution context
//   - req: MCP tool request
//   - input: input data specifying the directory for analysis
//
// Returns:
//   - MCP tool call result
//   - function complexity analysis result
//   - error if an error occurred while loading packages
func AnalyzeComplexity(ctx context.Context, req *mcp.CallToolRequest, input AnalyzeComplexityInput) (
	*mcp.CallToolResult,
	AnalyzeComplexityOutput,
	error,
) {
	start := logStart("AnalyzeComplexity", map[string]string{"dir": input.Dir})
	out := AnalyzeComplexityOutput{}

	defer func() { logEnd("AnalyzeComplexity", start, len(out.Functions)) }()

	mode := packages.NeedSyntax | packages.NeedTypes | packages.NeedCompiledGoFiles | packages.NeedTypesInfo

	pkgs, err := loadPackagesWithCache(ctx, input.Dir, mode)
	if err != nil {
		logError("AnalyzeComplexity", err, "failed to load packages")

		return fail(out, err)
	}

	functions := make([]FunctionComplexity, 0)

	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			absPath := pkg.Fset.File(file.Pos()).Name()
			relPath, _ := filepath.Rel(input.Dir, absPath)

			ast.Inspect(file, func(n ast.Node) bool {
				fd, ok := n.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					return true
				}

				pos := pkg.Fset.Position(fd.Pos())
				lines := pkg.Fset.Position(fd.End()).Line - pos.Line
				visitor := &ComplexityVisitor{
					Ctx: ctx, Fset: pkg.Fset, Nesting: 0, MaxNesting: 0, Cyclomatic: 1,
				}
				ast.Walk(visitor, fd.Body)
				functions = append(functions, FunctionComplexity{
					Name: fd.Name.Name, File: relPath, Line: pos.Line,
					Lines: lines, Nesting: visitor.MaxNesting, Cyclomatic: visitor.Cyclomatic,
				})

				return true
			})
		}
	}

	out.Functions = groupFunctionComplexityByFile(functions)

	return nil, out, nil
}

type ComplexityVisitor struct {
	Ctx        context.Context
	Fset       *token.FileSet
	Nesting    int
	MaxNesting int
	Cyclomatic int
}

func (v *ComplexityVisitor) Visit(n ast.Node) ast.Visitor {
	if shouldStop(v.Ctx) {
		return nil
	}

	switch n.(type) {
	case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt,
		*ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
		v.Nesting++
		if v.Nesting > v.MaxNesting {
			v.MaxNesting = v.Nesting
		}

		v.Cyclomatic++
		// возвращаем «scoped visitor», который после обхода уменьшит nesting
		return &scopedVisitor{v}
	case *ast.CaseClause:
		v.Cyclomatic++
	}

	return v
}

type scopedVisitor struct {
	parent *ComplexityVisitor
}

func (s *scopedVisitor) Visit(n ast.Node) ast.Visitor {
	if n == nil {
		// выход из поддерева — уменьшаем nesting
		s.parent.Nesting--

		return nil
	}

	return s.parent.Visit(n)
}

// MetricsSummary aggregates general project information: package/struct/interface counts,
// average cyclomatic complexity, unused code ratios.
//
// Parameters:
//   - ctx: execution context
//   - req: MCP tool request
//   - input: input data specifying the directory for analysis
//
// Returns:
//   - MCP tool call result
//   - aggregated project metrics
//   - error if an error occurred while loading packages
func MetricsSummary(ctx context.Context, req *mcp.CallToolRequest, input MetricsSummaryInput) (
	*mcp.CallToolResult,
	MetricsSummaryOutput,
	error,
) {
	start := logStart("MetricsSummary", map[string]string{"dir": input.Dir})
	out := MetricsSummaryOutput{}

	defer func() { logEnd("MetricsSummary", start, 0) }()

	mode := packages.NeedSyntax | packages.NeedTypes | packages.NeedCompiledGoFiles | packages.NeedTypesInfo | packages.NeedName

	pkgs, err := loadPackagesWithCache(ctx, input.Dir, mode)
	if err != nil {
		logError("MetricsSummary", err, "failed to load packages")

		return fail(out, err)
	}

	// Count packages
	out.PackageCount = len(pkgs)

	// Initialize counters
	var (
		totalCyclomatic int
		functionCount   int
		structCount     int
		interfaceCount  int
		lineCount       int
		fileCount       int
	)

	// Count symbols and calculate complexity

	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			absPath := pkg.Fset.File(file.Pos()).Name()

			// Count lines in this file
			content, err := os.ReadFile(absPath)
			if err == nil {
				lines := strings.Split(string(content), "\n")
				lineCount += len(lines)
				fileCount++
			}

			ast.Inspect(file, func(n ast.Node) bool {
				switch decl := n.(type) {
				case *ast.FuncDecl:
					// Count function and calculate its complexity
					functionCount++

					if decl.Body != nil {
						visitor := &ComplexityVisitor{
							Ctx: ctx, Fset: pkg.Fset, Cyclomatic: 1,
						}
						ast.Walk(visitor, decl.Body)
						totalCyclomatic += visitor.Cyclomatic
					}
				case *ast.TypeSpec:
					switch decl.Type.(type) {
					case *ast.StructType:
						structCount++
					case *ast.InterfaceType:
						interfaceCount++
					}
				}

				return true
			})
		}
	}

	out.FunctionCount = functionCount
	out.StructCount = structCount
	out.InterfaceCount = interfaceCount
	out.LineCount = lineCount
	out.FileCount = fileCount

	// Calculate average cyclomatic complexity
	if functionCount > 0 {
		out.AverageCyclomatic = float64(totalCyclomatic) / float64(functionCount)
	} else {
		out.AverageCyclomatic = 0
	}

	// Count dead code using existing DeadCode logic
	deadCodeInput := DeadCodeInput{Dir: input.Dir}

	_, deadCodeOutput, err := DeadCode(ctx, req, deadCodeInput)
	if err == nil {
		out.DeadCodeCount = len(deadCodeOutput.Unused)
	}

	// Count exported but unused symbols separately would require additional analysis
	// For now, we can approximate by checking exported symbols in the dead code output
	exportedUnused := 0

	for _, unused := range deadCodeOutput.Unused {
		if ast.IsExported(unused.Name) {
			exportedUnused++
		}
	}

	out.ExportedUnusedCount = exportedUnused

	return nil, out, nil
}
