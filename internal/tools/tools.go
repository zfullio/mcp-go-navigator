package tools

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/pmezard/go-difflib/difflib"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
)

func ListPackages(ctx context.Context, req *mcp.CallToolRequest, input ListPackagesInput) (
	*mcp.CallToolResult,
	ListPackagesOutput,
	error,
) {
	start := logStart("ListPackages", map[string]string{"dir": input.Dir})
	out := ListPackagesOutput{Packages: []string{}}

	defer func() { logEnd("ListPackages", start, len(out.Packages)) }()

	cfg := &packages.Config{
		Mode:    packages.NeedName | packages.NeedCompiledGoFiles,
		Dir:     input.Dir,
		Context: ctx,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		logError("ListPackages", err, "failed to load packages")

		return fail(out, err)
	}

	for _, pkg := range pkgs {
		out.Packages = append(out.Packages, pkg.PkgPath)
	}

	return nil, out, nil
}

func ListSymbols(ctx context.Context, req *mcp.CallToolRequest, input ListSymbolsInput) (
	*mcp.CallToolResult,
	ListSymbolsOutput,
	error,
) {
	start := logStart("ListSymbols", map[string]string{"dir": input.Dir})
	out := ListSymbolsOutput{Symbols: []Symbol{}}

	defer func() { logEnd("ListSymbols", start, len(out.Symbols)) }()

	mode := packages.NeedSyntax | packages.NeedTypes | packages.NeedCompiledGoFiles | packages.NeedTypesInfo | packages.NeedName

	pkgs, err := loadPackagesWithCache(ctx, input.Dir, mode)
	if err != nil {
		logError("ListSymbols", err, "failed to load packages")

		return fail(out, err)
	}

	for _, pkg := range pkgs {
		pkgPath := pkg.PkgPath
		if pkgPath == "" {
			pkgPath = pkg.Name
		}

		if input.Package != "" && pkgPath != input.Package && pkg.Name != input.Package {
			continue
		}

		for _, file := range pkg.Syntax {
			absPath := pkg.Fset.File(file.Pos()).Name()
			relPath, _ := filepath.Rel(input.Dir, absPath)

			ast.Inspect(file, func(n ast.Node) bool {
				switch decl := n.(type) {
				case *ast.FuncDecl:
					out.Symbols = append(out.Symbols, Symbol{
						Kind:     "func",
						Name:     decl.Name.Name,
						Package:  pkg.PkgPath,
						File:     relPath,
						Line:     pkg.Fset.Position(decl.Pos()).Line,
						Exported: decl.Name.IsExported(),
					})
				case *ast.TypeSpec:
					switch t := decl.Type.(type) {
					case *ast.StructType:
						out.Symbols = append(out.Symbols, Symbol{
							Kind:     "struct",
							Name:     decl.Name.Name,
							Package:  pkg.PkgPath,
							File:     relPath,
							Line:     pkg.Fset.Position(decl.Pos()).Line,
							Exported: decl.Name.IsExported(),
						})
					case *ast.InterfaceType:
						out.Symbols = append(out.Symbols, Symbol{
							Kind:     "interface",
							Name:     decl.Name.Name,
							Package:  pkg.PkgPath,
							File:     relPath,
							Line:     pkg.Fset.Position(decl.Pos()).Line,
							Exported: decl.Name.IsExported(),
						})

						for _, m := range t.Methods.List {
							if len(m.Names) > 0 {
								out.Symbols = append(out.Symbols, Symbol{
									Kind:     "method",
									Name:     decl.Name.Name + "." + m.Names[0].Name,
									Package:  pkg.PkgPath,
									File:     relPath,
									Line:     pkg.Fset.Position(m.Pos()).Line,
									Exported: m.Names[0].IsExported(),
								})
							}
						}
					}
				}

				return true
			})
		}
	}

	sort.Slice(out.Symbols, func(i, j int) bool {
		if out.Symbols[i].Package == out.Symbols[j].Package {
			return out.Symbols[i].Name < out.Symbols[j].Name
		}

		return out.Symbols[i].Package < out.Symbols[j].Package
	})

	return nil, out, nil
}

func FindReferences(ctx context.Context, req *mcp.CallToolRequest, input FindReferencesInput) (
	*mcp.CallToolResult,
	FindReferencesOutput,
	error,
) {
	start := logStart("FindReferences", map[string]string{
		"dir": input.Dir, "ident": input.Ident, "kind": input.Kind,
	})
	out := FindReferencesOutput{References: []Reference{}}

	defer func() { logEnd("FindReferences", start, len(out.References)) }()

	mode := packages.NeedSyntax | packages.NeedCompiledGoFiles | packages.NeedTypes | packages.NeedTypesInfo

	pkgs, err := loadPackagesWithCache(ctx, input.Dir, mode)
	if err != nil {
		logError("FindReferences", err, "failed to load packages")

		return fail(out, err)
	}

	// Find the target object to find references for
	var targetObj types.Object

	for _, pkg := range pkgs {
		if shouldStop(ctx) {
			return fail(out, context.Canceled)
		}

		// First check the package scope
		scope := pkg.Types.Scope()
		if scope != nil {
			if obj := scope.Lookup(input.Ident); obj != nil {
				if input.Kind == "" || objStringKind(obj) == input.Kind {
					targetObj = obj
					break
				}
			}
		}

		// Then check all definitions
		for ident, def := range pkg.TypesInfo.Defs {
			if shouldStop(ctx) {
				return fail(out, context.Canceled)
			}

			if def != nil && ident.Name == input.Ident {
				if input.Kind == "" || objStringKind(def) == input.Kind {
					targetObj = def
					break
				}
			}
		}

		if targetObj != nil {
			break
		}
	}

	if targetObj == nil {
		return nil, out, fmt.Errorf("symbol %q not found", input.Ident)
	}

	for _, pkg := range pkgs {
		if shouldStop(ctx) {
			return fail(out, context.Canceled)
		}

		for _, file := range pkg.Syntax {
			if shouldStop(ctx) {
				return fail(out, context.Canceled)
			}

			absPath := pkg.Fset.File(file.Pos()).Name()
			relPath, _ := filepath.Rel(input.Dir, absPath)
			lines := getFileLines(pkg.Fset, file)

			ast.Inspect(file, func(n ast.Node) bool {
				if shouldStop(ctx) {
					return false
				}

				ident, ok := n.(*ast.Ident)
				if !ok || ident.Name != input.Ident {
					return true
				}

				obj := pkg.TypesInfo.ObjectOf(ident)
				if obj == nil {
					return true
				}

				// Check if the object kind matches if specified
				if input.Kind != "" && input.Kind != objStringKind(obj) {
					return true
				}

				// Check if this reference is for our target object
				if !sameObject(obj, targetObj) {
					return true
				}

				pos := pkg.Fset.Position(ident.Pos())
				if pos.Filename == "" {
					return true
				}

				// Check if we need to filter by file
				if input.File != "" && !strings.HasSuffix(pos.Filename, input.File) {
					return true
				}

				snip := extractSnippet(lines, pos.Line)
				out.References = append(out.References, Reference{
					File: relPath, Line: pos.Line, Snippet: snip,
				})

				return true
			})
		}
	}

	return nil, out, nil
}

func FindDefinitions(ctx context.Context, req *mcp.CallToolRequest, input FindDefinitionsInput) (
	*mcp.CallToolResult,
	FindDefinitionsOutput,
	error,
) {
	start := logStart("FindDefinitions", map[string]string{
		"dir": input.Dir, "ident": input.Ident, "kind": input.Kind,
	})
	out := FindDefinitionsOutput{Definitions: []Definition{}}

	defer func() { logEnd("FindDefinitions", start, len(out.Definitions)) }()

	mode := packages.NeedSyntax | packages.NeedCompiledGoFiles | packages.NeedTypes | packages.NeedTypesInfo

	pkgs, err := loadPackagesWithCache(ctx, input.Dir, mode)
	if err != nil {
		logError("FindDefinitions", err, "failed to load packages")

		return fail(out, err)
	}

	for _, pkg := range pkgs {
		if shouldStop(ctx) {
			return fail(out, context.Canceled)
		}

		// First check the package scope
		scope := pkg.Types.Scope()
		if scope != nil {
			obj := scope.Lookup(input.Ident)
			if obj != nil {
				// Check if the object kind matches if specified
				if input.Kind == "" || objStringKind(obj) == input.Kind {
					pos := pkg.Fset.Position(obj.Pos())
					if pos.Filename != "" {
						// Check if we need to filter by file
						if input.File == "" || strings.HasSuffix(pos.Filename, input.File) {
							rel, _ := filepath.Rel(input.Dir, pos.Filename)
							lines := getFileLinesFromPath(pos.Filename)
							snippet := extractSnippet(lines, pos.Line)
							out.Definitions = append(out.Definitions, Definition{
								File: rel, Line: pos.Line, Snippet: snippet,
							})
						}
					}
				}
			}
		}

		// Then check all definitions in the package
		for ident, obj := range pkg.TypesInfo.Defs {
			if shouldStop(ctx) {
				return fail(out, context.Canceled)
			}

			if obj != nil && ident.Name == input.Ident {
				// Check if the object kind matches if specified
				if input.Kind == "" || objStringKind(obj) == input.Kind {
					pos := pkg.Fset.Position(obj.Pos())
					if pos.Filename != "" {
						// Check if we need to filter by file
						if input.File == "" || strings.HasSuffix(pos.Filename, input.File) {
							rel, _ := filepath.Rel(input.Dir, pos.Filename)
							lines := getFileLinesFromPath(pos.Filename)
							snippet := extractSnippet(lines, pos.Line)
							out.Definitions = append(out.Definitions, Definition{
								File: rel, Line: pos.Line, Snippet: snippet,
							})
						}
					}
				}
			}
		}
	}

	return nil, out, nil
}

func RenameSymbol(ctx context.Context, req *mcp.CallToolRequest, input RenameSymbolInput) (
	*mcp.CallToolResult,
	RenameSymbolOutput,
	error,
) {
	start := logStart("RenameSymbol", map[string]string{
		"dir": input.Dir, "oldName": input.OldName, "newName": input.NewName, "dryRun": strconv.FormatBool(input.DryRun),
	})
	out := RenameSymbolOutput{}

	defer func() { logEnd("RenameSymbol", start, len(out.ChangedFiles)) }()

	if input.OldName == input.NewName {
		out.Collisions = append(out.Collisions, fmt.Sprintf("cannot rename: %q == %q", input.OldName, input.NewName))

		return nil, out, nil
	}

	cfg := &packages.Config{
		Mode:    packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedCompiledGoFiles,
		Dir:     input.Dir,
		Context: ctx,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		logError("RenameSymbol", err, "failed to load packages")

		return fail(out, err)
	}

	// Find the target object to rename
	var targetObj types.Object
	for _, pkg := range pkgs {
		if shouldStop(ctx) {
			return fail(out, context.Canceled)
		}

		// Look in scope first
		scope := pkg.Types.Scope()
		if scope != nil {
			if obj := scope.Lookup(input.OldName); obj != nil {
				targetObj = obj
				break
			}
		}

		// Then look in defs
		for _, def := range pkg.TypesInfo.Defs {
			if def != nil && def.Name() == input.OldName {
				if input.Kind == "" || objStringKind(def) == input.Kind {
					targetObj = def
					break
				}
			}
		}

		if targetObj != nil {
			break
		}
	}

	if targetObj == nil {
		return nil, out, fmt.Errorf("symbol %q not found", input.OldName)
	}

	for _, pkg := range pkgs {
		if shouldStop(ctx) {
			return fail(out, context.Canceled)
		}

		for i, file := range pkg.Syntax {
			if shouldStop(ctx) {
				return fail(out, context.Canceled)
			}

			filename := pkg.CompiledGoFiles[i]
			origBytes, _ := os.ReadFile(filename)
			changed := false

			ast.Inspect(file, func(n ast.Node) bool {
				if shouldStop(ctx) {
					return false
				}

				// Only rename identifiers that refer to our target object
				if ident, ok := n.(*ast.Ident); ok {
					if ident.Name == input.OldName {
						obj := pkg.TypesInfo.ObjectOf(ident)
						if obj != nil && sameObject(obj, targetObj) {
							ident.Name = input.NewName
							changed = true
						}
					}
				}

				return true
			})

			if !changed {
				continue
			}

			var buf bytes.Buffer

			err := format.Node(&buf, pkg.Fset, file)
			if err != nil {
				logError("RenameSymbol", err, "failed to format file")

				return fail(out, err)
			}

			newContent := buf.Bytes()
			if len(newContent) > 0 && newContent[len(newContent)-1] != '\n' {
				newContent = append(newContent, '\n')
			}

			rel, _ := filepath.Rel(input.Dir, filename)
			out.ChangedFiles = append(out.ChangedFiles, rel)

			if input.DryRun {
				diff := difflib.UnifiedDiff{
					A:        difflib.SplitLines(string(origBytes)),
					B:        difflib.SplitLines(string(newContent)),
					FromFile: rel + " (old)",
					ToFile:   rel + " (new)",
					Context:  3,
				}
				text, _ := difflib.GetUnifiedDiffString(diff)
				out.Diffs = append(out.Diffs, FileDiff{Path: rel, Diff: text})

				continue
			}

			err = safeWriteFile(filename, newContent)
			if err != nil {
				logError("RenameSymbol", err, "failed to write file")

				return fail(out, err)
			}
		}
	}

	return nil, out, nil
}

func ListImports(ctx context.Context, req *mcp.CallToolRequest, input ListImportsInput) (
	*mcp.CallToolResult,
	ListImportsOutput,
	error,
) {
	start := logStart("ListImports", map[string]string{"dir": input.Dir})
	out := ListImportsOutput{}

	defer func() { logEnd("ListImports", start, len(out.Imports)) }()

	mode := packages.NeedSyntax | packages.NeedCompiledGoFiles

	pkgs, err := loadPackagesWithCache(ctx, input.Dir, mode)
	if err != nil {
		logError("ListImports", err, "failed to load packages")

		return fail(out, err)
	}

	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			absPath := pkg.Fset.File(file.Pos()).Name()

			relPath, _ := filepath.Rel(input.Dir, absPath)
			for _, imp := range file.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				pos := pkg.Fset.Position(imp.Pos())
				out.Imports = append(out.Imports, Import{Path: path, File: relPath, Line: pos.Line})
			}
		}
	}

	return nil, out, nil
}

func ListInterfaces(ctx context.Context, req *mcp.CallToolRequest, input ListInterfacesInput) (
	*mcp.CallToolResult,
	ListInterfacesOutput,
	error,
) {
	start := logStart("ListInterfaces", map[string]string{"dir": input.Dir})
	out := ListInterfacesOutput{}

	defer func() { logEnd("ListInterfaces", start, len(out.Interfaces)) }()

	mode := packages.NeedSyntax | packages.NeedCompiledGoFiles

	pkgs, err := loadPackagesWithCache(ctx, input.Dir, mode)
	if err != nil {
		logError("ListInterfaces", err, "failed to load packages")

		return fail(out, err)
	}

	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			absPath := pkg.Fset.File(file.Pos()).Name()
			relPath, _ := filepath.Rel(input.Dir, absPath)

			ast.Inspect(file, func(n ast.Node) bool {
				ts, ok := n.(*ast.TypeSpec)
				if !ok {
					return true
				}

				if iface, ok := ts.Type.(*ast.InterfaceType); ok {
					pos := symbolPos(pkg, ts)

					ifInfo := InterfaceInfo{
						Name: ts.Name.Name, File: relPath, Line: pos.Line, Methods: []InterfaceMethod{},
					}
					for _, m := range iface.Methods.List {
						if len(m.Names) > 0 {
							ifInfo.Methods = append(ifInfo.Methods, InterfaceMethod{
								Name: m.Names[0].Name, Line: pkg.Fset.Position(m.Pos()).Line,
							})
						}
					}

					out.Interfaces = append(out.Interfaces, ifInfo)
				}

				return true
			})
		}
	}

	return nil, out, nil
}

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
				out.Functions = append(out.Functions, FunctionComplexity{
					Name: fd.Name.Name, File: relPath, Line: pos.Line,
					Lines: lines, Nesting: visitor.MaxNesting, Cyclomatic: visitor.Cyclomatic,
				})

				return true
			})
		}
	}

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

	cfg := &packages.Config{
		Mode:    packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedCompiledGoFiles | packages.NeedName,
		Dir:     input.Dir,
		Context: ctx,
	}

	pkgs, err := packages.Load(cfg, "./...")
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

	cfg := &packages.Config{
		Mode:    packages.NeedName | packages.NeedImports | packages.NeedCompiledGoFiles,
		Dir:     input.Dir,
		Context: ctx,
	}

	pkgs, err := packages.Load(cfg, "./...")
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

func FindImplementations(ctx context.Context, req *mcp.CallToolRequest, input FindImplementationsInput) (
	*mcp.CallToolResult,
	FindImplementationsOutput,
	error,
) {
	start := logStart("FindImplementations", map[string]string{"dir": input.Dir, "name": input.Name})
	out := FindImplementationsOutput{Implementations: []Implementation{}}

	defer func() { logEnd("FindImplementations", start, len(out.Implementations)) }()

	mode := packages.NeedSyntax | packages.NeedTypes | packages.NeedCompiledGoFiles | packages.NeedTypesInfo

	pkgs, err := loadPackagesWithCache(ctx, input.Dir, mode)
	if err != nil {
		logError("FindImplementations", err, "failed to load packages")

		return fail(out, err)
	}

	// Find the target interface/type in the type information
	var (
		targetObj      types.Object
		targetTypeName string
	)

	for _, pkg := range pkgs {
		scope := pkg.Types.Scope()
		if scope != nil {
			obj := scope.Lookup(input.Name)
			if obj != nil && obj.Pkg() != nil {
				targetObj = obj
				targetTypeName = obj.Type().String()

				break
			}
		}
	}

	if targetObj == nil {
		// Look in all definitions as well
		for _, pkg := range pkgs {
			for _, def := range pkg.TypesInfo.Defs {
				if def != nil && def.Name() == input.Name {
					targetObj = def
					targetTypeName = def.Type().String()

					break
				}
			}

			if targetObj != nil {
				break
			}
		}
	}

	if targetObj == nil {
		return nil, out, fmt.Errorf("interface or type %q not found", input.Name)
	}

	// Verify that the target is an interface
	targetType, ok := targetObj.Type().Underlying().(*types.Interface)
	if !ok {
		return nil, out, fmt.Errorf("%q is not an interface", input.Name)
	}

	// Look for types that implement this interface
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			absPath := pkg.Fset.File(file.Pos()).Name()
			relPath, _ := filepath.Rel(input.Dir, absPath)

			// Find all type declarations and check if they implement the interface
			ast.Inspect(file, func(n ast.Node) bool {
				switch decl := n.(type) {
				case *ast.TypeSpec:
					if decl.Name.Name == input.Name {
						// This is the target interface itself, skip it
						return true
					}

					// Get the type from types info
					if obj := pkg.TypesInfo.Defs[decl.Name]; obj != nil {
						typ := obj.Type()
						if typ != nil && types.Implements(typ, targetType) {
							// Type implements the interface
							pos := pkg.Fset.Position(decl.Pos())
							out.Implementations = append(out.Implementations, Implementation{
								Type:      typ.String(),
								Interface: targetTypeName,
								File:      relPath,
								Line:      pos.Line,
								IsType:    true,
							})
						} else if iface, ok := typ.Underlying().(*types.Interface); ok && iface != targetType {
							// Check if it's another interface that extends the target one
							if sameInterface(iface, targetType) || interfaceExtends(iface, targetType) {
								pos := pkg.Fset.Position(decl.Pos())
								out.Implementations = append(out.Implementations, Implementation{
									Type:      typ.String(),
									Interface: targetTypeName,
									File:      relPath,
									Line:      pos.Line,
									IsType:    false,
								})
							}
						}
					}
				}

				return true
			})
		}
	}

	return nil, out, nil
}

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

func ASTRewrite(ctx context.Context, req *mcp.CallToolRequest, input ASTRewriteInput) (
	*mcp.CallToolResult,
	ASTRewriteOutput,
	error,
) {
	start := logStart("ASTRewrite", map[string]string{
		"dir": input.Dir, "find": input.Find, "replace": input.Replace,
	})
	out := ASTRewriteOutput{}

	defer func() { logEnd("ASTRewrite", start, out.TotalChanges) }()

	// Parse find and replace expressions once
	findExpr, err := parser.ParseExpr(input.Find)
	if err != nil {
		return nil, out, fmt.Errorf("invalid find expression: %w", err)
	}

	replaceExpr, err := parser.ParseExpr(input.Replace)
	if err != nil {
		return nil, out, fmt.Errorf("invalid replace expression: %w", err)
	}

	cfg := &packages.Config{
		Mode:    packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedCompiledGoFiles,
		Dir:     input.Dir,
		Context: ctx,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		logError("ASTRewrite", err, "failed to load packages")

		return fail(out, err)
	}

	totalChanges := 0

	for _, pkg := range pkgs {
		for i, file := range pkg.Syntax {
			filename := pkg.CompiledGoFiles[i]
			origBytes, _ := os.ReadFile(filename)
			changesInFile := 0

			rewriter := &ASTRewriteVisitor{
				Fset:        pkg.Fset,
				TypesInfo:   pkg.TypesInfo,
				FindPattern: findExpr,
				ReplaceWith: replaceExpr,
				Changes:     &changesInFile,
			}

			newFile := ast.Node(rewriter.Rewrite(file))

			if changesInFile == 0 {
				continue
			}

			var buf bytes.Buffer

			err := format.Node(&buf, pkg.Fset, newFile)
			if err != nil {
				logError("ASTRewrite", err, "failed to format file")

				return fail(out, err)
			}

			newContent := buf.Bytes()
			if len(newContent) > 0 && newContent[len(newContent)-1] != '\n' {
				newContent = append(newContent, '\n')
			}

			rel, _ := filepath.Rel(input.Dir, filename)
			out.ChangedFiles = append(out.ChangedFiles, rel)
			totalChanges += changesInFile

			if input.DryRun {
				diff := difflib.UnifiedDiff{
					A:        difflib.SplitLines(string(origBytes)),
					B:        difflib.SplitLines(string(newContent)),
					FromFile: rel + " (old)",
					ToFile:   rel + " (new)",
					Context:  3,
				}
				text, _ := difflib.GetUnifiedDiffString(diff)
				out.Diffs = append(out.Diffs, FileDiff{Path: rel, Diff: text})
			} else {
				err := safeWriteFile(filename, newContent)
				if err != nil {
					logError("ASTRewrite", err, "failed to write file")

					return fail(out, err)
				}
			}
		}
	}

	out.TotalChanges = totalChanges

	return nil, out, nil
}

// ASTRewriteVisitor traverses the AST and rewrites matching nodes.
type ASTRewriteVisitor struct {
	Fset        *token.FileSet
	TypesInfo   *types.Info
	FindPattern ast.Expr
	ReplaceWith ast.Expr
	Changes     *int
}

// Rewrite walks through the AST and replaces matching expressions.
func (v *ASTRewriteVisitor) Rewrite(node ast.Node) ast.Node {
	return astutil.Apply(node, func(c *astutil.Cursor) bool {
		n := c.Node()
		if n == nil {
			return true
		}

		// Сравниваем только выражения
		expr, ok := n.(ast.Expr)
		if !ok {
			return true
		}

		// Сравниваем текущий узел с искомым паттерном
		if astEqual(expr, v.FindPattern) {
			*v.Changes++
			c.Replace(v.ReplaceWith)

			return false // не спускаться глубже
		}

		return true
	}, nil)
}

// astEqual compares two AST expressions structurally.
func astEqual(a, b ast.Expr) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Use the ast.Inspect function to walk both ASTs simultaneously for deep comparison
	// This is more efficient than formatting to strings
	return compareASTNodes(a, b)
}

// compareASTNodes compares two AST nodes recursively
func compareASTNodes(a, b ast.Node) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Use type switches to compare the specific node types
	switch aVal := a.(type) {
	case *ast.Ident:
		if bVal, ok := b.(*ast.Ident); ok {
			return aVal.Name == bVal.Name
		}
	case *ast.BasicLit:
		if bVal, ok := b.(*ast.BasicLit); ok {
			return aVal.Kind == bVal.Kind && aVal.Value == bVal.Value
		}
	case *ast.BinaryExpr:
		if bVal, ok := b.(*ast.BinaryExpr); ok {
			return aVal.Op == bVal.Op &&
				compareASTNodes(aVal.X, bVal.X) &&
				compareASTNodes(aVal.Y, bVal.Y)
		}
	case *ast.CallExpr:
		if bVal, ok := b.(*ast.CallExpr); ok {
			return compareASTNodes(aVal.Fun, bVal.Fun) &&
				compareExprSlices(aVal.Args, bVal.Args)
		}
	case *ast.SelectorExpr:
		if bVal, ok := b.(*ast.SelectorExpr); ok {
			return compareASTNodes(aVal.X, bVal.X) &&
				compareASTNodes(aVal.Sel, bVal.Sel)
		}
	case *ast.ParenExpr:
		if bVal, ok := b.(*ast.ParenExpr); ok {
			return compareASTNodes(aVal.X, bVal.X)
		}
	case *ast.StarExpr:
		if bVal, ok := b.(*ast.StarExpr); ok {
			return compareASTNodes(aVal.X, bVal.X)
		}
	case *ast.TypeAssertExpr:
		if bVal, ok := b.(*ast.TypeAssertExpr); ok {
			return compareASTNodes(aVal.X, bVal.X) &&
				compareASTNodes(aVal.Type, bVal.Type)
		}
	case *ast.IndexExpr:
		if bVal, ok := b.(*ast.IndexExpr); ok {
			return compareASTNodes(aVal.X, bVal.X) &&
				compareASTNodes(aVal.Index, bVal.Index)
		}
	case *ast.SliceExpr:
		if bVal, ok := b.(*ast.SliceExpr); ok {
			return compareASTNodes(aVal.X, bVal.X) &&
				compareASTNodes(aVal.Low, bVal.Low) &&
				compareASTNodes(aVal.High, bVal.High) &&
				compareASTNodes(aVal.Max, bVal.Max)
		}
	case *ast.FuncLit:
		if bVal, ok := b.(*ast.FuncLit); ok {
			// For FuncLit, we'll do a basic comparison of type and body since
			// comparing functions properly is complex
			return compareASTNodes(aVal.Type, bVal.Type) &&
				compareASTNodes(aVal.Body, bVal.Body)
		}
	case *ast.CompositeLit:
		if bVal, ok := b.(*ast.CompositeLit); ok {
			return compareASTNodes(aVal.Type, bVal.Type) &&
				compareExprSlices(aVal.Elts, bVal.Elts)
		}
	case *ast.KeyValueExpr:
		if bVal, ok := b.(*ast.KeyValueExpr); ok {
			return compareASTNodes(aVal.Key, bVal.Key) &&
				compareASTNodes(aVal.Value, bVal.Value)
		}
	case *ast.UnaryExpr:
		if bVal, ok := b.(*ast.UnaryExpr); ok {
			return aVal.Op == bVal.Op &&
				compareASTNodes(aVal.X, bVal.X)
		}
	// Add other cases as needed for more comprehensive comparison
	// For this implementation, if it's not one of the known types,
	// we'll fall back to a string comparison approach which is less efficient
	// but covers all cases
	default:
		var bufA, bufB bytes.Buffer
		fset := token.NewFileSet()
		_ = format.Node(&bufA, fset, a)
		_ = format.Node(&bufB, fset, b)
		return bufA.String() == bufB.String()
	}

	return false
}

// compareExprSlices compares two slices of expressions
func compareExprSlices(a, b []ast.Expr) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if !compareASTNodes(a[i], b[i]) {
			return false
		}
	}

	return true
}
