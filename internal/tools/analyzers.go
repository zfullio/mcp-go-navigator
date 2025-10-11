package tools

import (
	"context"
	"go/ast"
	"go/token"
	"go/types"
	"os"
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
	start := logStart("DeadCode", logFields(
		input.Dir,
		newLogField("package", input.Package),
	))
	out := DeadCodeOutput{
		Unused:    []DeadSymbol{},
		ByPackage: make(map[string]int),
	}

	defer func() { logEnd("DeadCode", start, len(out.Unused)) }()

	mode := loadModeSyntaxTypesNamed

	_, filteredPkgs, err := loadFilteredPackages(ctx, input.Dir, mode, input.Package, "DeadCode")
	if err != nil {
		return fail(out, err)
	}

	exportedCount := 0
	byKind := make(map[string]int)

	for _, pkg := range filteredPkgs {
		pkgKey := normalizePackagePath(pkg)
		if pkgKey == "" {
			pkgKey = pkg.Name
		}

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
			rel := relativePath(input.Dir, pos.Filename)

			symbol := DeadSymbol{
				Name:       ident.Name,
				Kind:       objStringKind(obj),
				File:       rel,
				Line:       pos.Line,
				IsExported: isExported,
				Package:    pkgKey,
			}

			out.Unused = append(out.Unused, symbol)

			if isExported {
				exportedCount++
			}

			// Update aggregated counters
			out.ByPackage[pkgKey]++
			byKind[symbol.Kind]++
		}
	}

	out.TotalCount = len(out.Unused)
	out.ExportedCount = exportedCount
	out.ByKind = byKind

	if input.Limit > 0 && len(out.Unused) > input.Limit {
		out.HasMore = true
		out.Unused = out.Unused[:input.Limit]
	}

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
	start := logStart("AnalyzeDependencies", logFields(
		input.Dir,
		newLogField("package", input.Package),
	))
	out := AnalyzeDependenciesOutput{
		Dependencies: []PackageDependency{},
		Cycles:       [][]string{},
	}

	defer func() { logEnd("AnalyzeDependencies", start, len(out.Dependencies)) }()

	mode := loadModeBasic | packages.NeedImports

	pkgs, filteredPkgs, err := loadFilteredPackages(ctx, input.Dir, mode, input.Package, "AnalyzeDependencies")
	if err != nil {
		return fail(out, err)
	}

	depGraph := make(map[string][]string)
	pkgMap := make(map[string]*packages.Package)
	fanIn := make(map[string]int)

	for _, pkg := range pkgs {
		key := normalizePackagePath(pkg)
		if key == "" {
			continue
		}

		pkgMap[key] = pkg

		for impPath := range pkg.Imports {
			depGraph[key] = append(depGraph[key], impPath)
			fanIn[impPath]++
		}
	}

	filteredKeys := make(map[string]struct{}, len(filteredPkgs))
	for _, pkg := range filteredPkgs {
		key := normalizePackagePath(pkg)
		if key != "" {
			filteredKeys[key] = struct{}{}
		}
	}

	for _, pkg := range filteredPkgs {
		key := normalizePackagePath(pkg)

		imports := make([]string, 0, len(pkg.Imports))
		for impPath := range pkg.Imports {
			imports = append(imports, impPath)
		}

		fanOut := len(imports)
		fanInCount := fanIn[key]

		out.Dependencies = append(out.Dependencies, PackageDependency{
			Package: key,
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

						includeCycle := len(filteredKeys) == 0
						if !includeCycle {
							for _, item := range cycle {
								if _, ok := filteredKeys[item]; ok {
									includeCycle = true

									break
								}
							}
						}

						if includeCycle {
							out.Cycles = append(out.Cycles, cycle)
						}

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
func AnalyzeComplexity(ctx context.Context, _ *mcp.CallToolRequest, input AnalyzeComplexityInput) (
	*mcp.CallToolResult,
	AnalyzeComplexityOutput,
	error,
) {
	start := logStart("AnalyzeComplexity", logFields(
		input.Dir,
		newLogField("package", input.Package),
	))
	out := AnalyzeComplexityOutput{}

	defer func() { logEnd("AnalyzeComplexity", start, len(out.Functions)) }()

	mode := loadModeSyntaxTypesNamed

	_, filteredPkgs, err := loadFilteredPackages(ctx, input.Dir, mode, input.Package, "AnalyzeComplexity")
	if err != nil {
		return fail(out, err)
	}

	functions := make([]FunctionComplexity, 0)

	if err := walkPackageFiles(ctx, filteredPkgs, input.Dir, func(pkg *packages.Package, file *ast.File, relPath string, _ int) error {
		ast.Inspect(file, func(n ast.Node) bool {
			fd, ok := n.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				return true
			}

			pos := pkg.Fset.Position(fd.Pos())
			lines, nesting, cyclomatic := computeFunctionMetrics(ctx, pkg.Fset, fd)
			functions = append(functions, FunctionComplexity{
				Name: fd.Name.Name, File: relPath, Line: pos.Line,
				Lines: lines, Nesting: nesting, Cyclomatic: cyclomatic,
			})

			return true
		})

		return nil
	}); err != nil {
		return fail(out, err)
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
	start := logStart("MetricsSummary", logFields(
		input.Dir,
		newLogField("package", input.Package),
	))
	out := MetricsSummaryOutput{}

	defer func() { logEnd("MetricsSummary", start, 0) }()

	mode := loadModeSyntaxTypesNamed

	_, filteredPkgs, err := loadFilteredPackages(ctx, input.Dir, mode, input.Package, "MetricsSummary")
	if err != nil {
		return fail(out, err)
	}

	// Count packages
	out.PackageCount = len(filteredPkgs)

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

	if err := walkPackageFiles(ctx, filteredPkgs, input.Dir, func(pkg *packages.Package, file *ast.File, _ string, idx int) error {
		absPath := pkg.Fset.File(file.Pos()).Name()
		if absPath == "" {
			if len(pkg.CompiledGoFiles) > idx {
				absPath = pkg.CompiledGoFiles[idx]
			} else if len(pkg.GoFiles) > idx {
				absPath = pkg.GoFiles[idx]
			}
		}

		if absPath != "" {
			if content, err := os.ReadFile(absPath); err == nil {
				lines := strings.Split(string(content), "\n")
				lineCount += len(lines)
				fileCount++
			}
		}

		ast.Inspect(file, func(n ast.Node) bool {
			switch decl := n.(type) {
			case *ast.FuncDecl:
				functionCount++

				if decl.Body != nil {
					_, _, cyclomatic := computeFunctionMetrics(ctx, pkg.Fset, decl)
					totalCyclomatic += cyclomatic
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

		return nil
	}); err != nil {
		return fail(out, err)
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
	if input.Package != "" {
		deadCodeInput.Package = input.Package
	}

	_, deadCodeOutput, err := DeadCode(ctx, req, deadCodeInput)
	if err == nil {
		out.DeadCodeCount = deadCodeOutput.TotalCount
		out.ExportedUnusedCount = deadCodeOutput.ExportedCount
	}

	return nil, out, nil
}
