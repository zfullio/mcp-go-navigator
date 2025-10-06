// Package tools provides a set of tools for analyzing and refactoring Go code using the Model Context Protocol (MCP).
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

// ListPackages returns all Go packages in the specified directory.
//
// Parameters:
//   - ctx: execution context
//   - req: MCP tool request
//   - input: input data specifying the directory to scan
//
// Returns:
//   - MCP tool call result
//   - list of found packages
//   - error if an error occurred while loading packages
func ListPackages(ctx context.Context, req *mcp.CallToolRequest, input ListPackagesInput) (
	*mcp.CallToolResult,
	ListPackagesOutput,
	error,
) {
	start := logStart("ListPackages", map[string]string{"dir": input.Dir})
	out := ListPackagesOutput{Packages: []string{}}

	defer func() { logEnd("ListPackages", start, len(out.Packages)) }()

	mode := packages.NeedName | packages.NeedCompiledGoFiles

	pkgs, err := loadPackagesWithCache(ctx, input.Dir, mode)
	if err != nil {
		logError("ListPackages", err, "failed to load packages")

		return fail(out, err)
	}

	for _, pkg := range pkgs {
		out.Packages = append(out.Packages, pkg.PkgPath)
	}

	return nil, out, nil
}

// ListSymbols returns a list of all functions, structs, interfaces and methods in a Go package.
//
// Parameters:
//   - ctx: execution context
//   - req: MCP tool request
//   - input: input data specifying the directory and package to scan
//
// Returns:
//   - MCP tool call result
//   - list of found symbols
//   - error if an error occurred while loading packages
func ListSymbols(ctx context.Context, req *mcp.CallToolRequest, input ListSymbolsInput) (
	*mcp.CallToolResult,
	ListSymbolsOutput,
	error,
) {
	start := logStart("ListSymbols", map[string]string{"dir": input.Dir})
	symbols := []Symbol{}

	defer func() { logEnd("ListSymbols", start, len(symbols)) }()

	mode := packages.NeedSyntax | packages.NeedTypes | packages.NeedCompiledGoFiles | packages.NeedTypesInfo | packages.NeedName

	pkgs, err := loadPackagesWithCache(ctx, input.Dir, mode)
	if err != nil {
		logError("ListSymbols", err, "failed to load packages")

		return fail(ListSymbolsOutput{}, err)
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
					symbols = append(symbols, Symbol{
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
						symbols = append(symbols, Symbol{
							Kind:     "struct",
							Name:     decl.Name.Name,
							Package:  pkg.PkgPath,
							File:     relPath,
							Line:     pkg.Fset.Position(decl.Pos()).Line,
							Exported: decl.Name.IsExported(),
						})
					case *ast.InterfaceType:
						symbols = append(symbols, Symbol{
							Kind:     "interface",
							Name:     decl.Name.Name,
							Package:  pkg.PkgPath,
							File:     relPath,
							Line:     pkg.Fset.Position(decl.Pos()).Line,
							Exported: decl.Name.IsExported(),
						})

						for _, m := range t.Methods.List {
							if len(m.Names) > 0 {
								symbols = append(symbols, Symbol{
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

	sort.Slice(symbols, func(i, j int) bool {
		if symbols[i].Package == symbols[j].Package {
			return symbols[i].Name < symbols[j].Name
		}

		return symbols[i].Package < symbols[j].Package
	})

	// Group symbols by package and file for token efficiency
	groupedSymbols := groupSymbolsByPackageAndFile(symbols)

	out := ListSymbolsOutput{
		GroupedSymbols: groupedSymbols,
	}

	return nil, out, nil
}

// groupSymbolsByPackageAndFile groups symbols by package and file for token efficiency.
func groupSymbolsByPackageAndFile(symbols []Symbol) []SymbolGroupByPackage {
	packageMap := make(map[string]map[string][]SymbolInfo)

	// Group symbols by package and file
	for _, sym := range symbols {
		if _, exists := packageMap[sym.Package]; !exists {
			packageMap[sym.Package] = make(map[string][]SymbolInfo)
		}

		symbolInfo := SymbolInfo{
			Kind:     sym.Kind,
			Name:     sym.Name,
			Line:     sym.Line,
			Exported: sym.Exported,
		}

		packageMap[sym.Package][sym.File] = append(packageMap[sym.Package][sym.File], symbolInfo)
	}

	// Convert to the grouped structure
	var result []SymbolGroupByPackage

	for pkgName, fileMap := range packageMap {
		var files []SymbolGroupByFile
		for fileName, syms := range fileMap {
			files = append(files, SymbolGroupByFile{
				File:    fileName,
				Symbols: syms,
			})
		}

		// Sort files by name for consistency
		sort.Slice(files, func(i, j int) bool {
			return files[i].File < files[j].File
		})

		result = append(result, SymbolGroupByPackage{
			Package: pkgName,
			Files:   files,
		})
	}

	// Sort packages by name for consistency
	sort.Slice(result, func(i, j int) bool {
		return result[i].Package < result[j].Package
	})

	return result
}

// groupFunctionComplexityByFile groups functions by file for token efficiency.
func groupFunctionComplexityByFile(functions []FunctionComplexity) []FunctionComplexityGroupByFile {
	fileMap := make(map[string][]FunctionComplexityInfo)

	// Group symbols by package and file
	for _, fn := range functions {
		if _, exists := fileMap[fn.File]; !exists {
			fileMap[fn.File] = make([]FunctionComplexityInfo, 0)
		}

		functionInfo := FunctionComplexityInfo{
			Name:       fn.Name,
			Line:       fn.Line,
			Lines:      fn.Lines,
			Nesting:    fn.Nesting,
			Cyclomatic: fn.Cyclomatic,
		}

		fileMap[fn.File] = append(fileMap[fn.File], functionInfo)
	}

	// Convert to the grouped structure
	var result []FunctionComplexityGroupByFile

	for fileName, fns := range fileMap {
		// Sort files by name for consistency
		sort.Slice(fns, func(i, j int) bool {
			return fns[i].Line < fns[j].Line
		})

		result = append(result, FunctionComplexityGroupByFile{
			File:      fileName,
			Functions: fns,
		})
	}

	// Sort packages by name for consistency
	sort.Slice(result, func(i, j int) bool {
		return result[i].File < result[j].File
	})

	return result
}

// groupImportsByFile groups imports by file to reduce duplication.
func groupImportsByFile(imports []Import) []ImportGroupByFile {
	if len(imports) == 0 {
		return nil
	}

	fileMap := make(map[string][]ImportInfo)

	for _, imp := range imports {
		info := ImportInfo{Path: imp.Path, Line: imp.Line}
		fileMap[imp.File] = append(fileMap[imp.File], info)
	}

	var result []ImportGroupByFile

	for fileName, infos := range fileMap {
		sort.Slice(infos, func(i, j int) bool {
			if infos[i].Path == infos[j].Path {
				return infos[i].Line < infos[j].Line
			}

			return infos[i].Path < infos[j].Path
		})

		result = append(result, ImportGroupByFile{
			File:    fileName,
			Imports: infos,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].File < result[j].File
	})

	return result
}

// groupInterfacesByPackage groups interfaces by package.
func groupInterfacesByPackage(data map[string][]InterfaceInfo) []InterfaceGroupByPackage {
	if len(data) == 0 {
		return nil
	}

	var result []InterfaceGroupByPackage

	for pkgName, interfaces := range data {
		sort.Slice(interfaces, func(i, j int) bool {
			return interfaces[i].Name < interfaces[j].Name
		})

		result = append(result, InterfaceGroupByPackage{
			Package:    pkgName,
			Interfaces: interfaces,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Package < result[j].Package
	})

	return result
}

// FindReferences finds all references and uses of an identifier using go/types semantic analysis.
//
// Parameters:
//   - ctx: execution context
//   - req: MCP tool request
//   - input: input data specifying the directory, identifier to search for, and filters
//
// Returns:
//   - MCP tool call result
//   - list of found references
//   - error if the symbol is not found or another error occurred
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
		return fail(out, err)
	}

	target := findTargetObject(ctx, pkgs, input.Ident, input.Kind)
	if target == nil {
		return nil, out, fmt.Errorf("symbol %q not found", input.Ident)
	}

	for _, pkg := range pkgs {
		if shouldStop(ctx) {
			return fail(out, context.Canceled)
		}

		for _, file := range pkg.Syntax {
			absPath := pkg.Fset.File(file.Pos()).Name()
			lines := getFileLines(pkg.Fset, file)

			ast.Inspect(file, func(n ast.Node) bool {
				ident, ok := n.(*ast.Ident)
				if !ok || ident.Name != input.Ident {
					return true
				}

				obj := pkg.TypesInfo.ObjectOf(ident)
				if obj == nil || (input.Kind != "" && input.Kind != objStringKind(obj)) {
					return true
				}

				if !sameObject(obj, target) {
					return true
				}

				pos := pkg.Fset.Position(ident.Pos())
				if pos.Filename == "" {
					return true
				}

				if input.File != "" && !strings.HasSuffix(pos.Filename, input.File) {
					return true
				}

				snip := extractSnippet(lines, pos.Line)
				appendReference(&out.References, input.Dir, absPath, pos.Line, snip)

				return true
			})
		}
	}

	return nil, out, nil
}

// FindDefinitions locates where a symbol is defined (type, func, var, const).
//
// Parameters:
//   - ctx: execution context
//   - req: MCP tool request
//   - input: input data specifying the directory and identifier to search for
//
// Returns:
//   - MCP tool call result
//   - list of found definitions
//   - error if an error occurred while loading packages
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
		return fail(out, err)
	}

	for _, pkg := range pkgs {
		if shouldStop(ctx) {
			return fail(out, context.Canceled)
		}

		if obj := findTargetObject(ctx, []*packages.Package{pkg}, input.Ident, input.Kind); obj != nil {
			appendDefinition(&out.Definitions, input.Dir, pkg.Fset, obj.Pos(), input.File)
		}
	}

	return nil, out, nil
}

// RenameSymbol performs a safe, scope-aware rename with dry-run diff preview.
//
// Parameters:
//   - ctx: execution context
//   - req: MCP tool request
//   - input: input data specifying the directory, old and new symbol names
//
// Returns:
//   - MCP tool call result
//   - rename result with information about changed files
//   - error if an error occurred while loading packages or the symbol was not found
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

	mode := packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedCompiledGoFiles

	pkgs, err := loadPackagesWithCache(ctx, input.Dir, mode)
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
					FromFile: "a/" + rel,
					ToFile:   "b/" + rel,
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

// ListImports returns a list of all imported packages in Go files in the specified directory.
//
// Parameters:
//   - ctx: execution context
//   - req: MCP tool request
//   - input: input data specifying the directory to scan
//
// Returns:
//   - MCP tool call result
//   - list of found imports
//   - error if an error occurred while loading packages
func ListImports(ctx context.Context, req *mcp.CallToolRequest, input ListImportsInput) (
	*mcp.CallToolResult,
	ListImportsOutput,
	error,
) {
	start := logStart("ListImports", map[string]string{"dir": input.Dir})
	out := ListImportsOutput{}

	defer func() { logEnd("ListImports", start, len(out.Imports)) }()

	mode := packages.NeedSyntax | packages.NeedCompiledGoFiles | packages.NeedName

	pkgs, err := loadPackagesWithCache(ctx, input.Dir, mode)
	if err != nil {
		logError("ListImports", err, "failed to load packages")

		return fail(out, err)
	}

	flatImports := make([]Import, 0)

	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			absPath := pkg.Fset.File(file.Pos()).Name()

			relPath, _ := filepath.Rel(input.Dir, absPath)
			for _, imp := range file.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				pos := pkg.Fset.Position(imp.Pos())
				flatImports = append(flatImports, Import{Path: path, File: relPath, Line: pos.Line})
			}
		}
	}

	out.Imports = groupImportsByFile(flatImports)

	return nil, out, nil
}

// ListInterfaces returns a list of all interfaces and their methods for dependency analysis or mocking.
//
// Parameters:
//   - ctx: execution context
//   - req: MCP tool request
//   - input: input data specifying the directory to scan
//
// Returns:
//   - MCP tool call result
//   - list of found interfaces
//   - error if an error occurred while loading packages
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

	interfacesByPackage := make(map[string][]InterfaceInfo)

	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			absPath := pkg.Fset.File(file.Pos()).Name()
			relPath, _ := filepath.Rel(input.Dir, absPath)

			pkgKey := pkg.PkgPath
			if pkgKey == "" {
				pkgKey = pkg.Name
			}

			if pkgKey == "" && file.Name != nil {
				pkgKey = file.Name.Name
			}

			if pkgKey == "" {
				pkgKey = "(unknown)"
			}

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
					if iface.Methods != nil {
						for _, m := range iface.Methods.List {
							if len(m.Names) > 0 {
								ifInfo.Methods = append(ifInfo.Methods, InterfaceMethod{
									Name: m.Names[0].Name, Line: pkg.Fset.Position(m.Pos()).Line,
								})
							}
						}
					}

					interfacesByPackage[pkgKey] = append(interfacesByPackage[pkgKey], ifInfo)
				}

				return true
			})
		}
	}

	out.Interfaces = groupInterfacesByPackage(interfacesByPackage)

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

// FindImplementations shows which concrete types implement interfaces (and vice versa).
//
// Parameters:
//   - ctx: execution context
//   - req: MCP tool request
//   - input: input data specifying the directory and interface name to search for
//
// Returns:
//   - MCP tool call result
//   - list of found implementations
//   - error if the interface is not found or another error occurred
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

// ASTRewrite allows replacing AST nodes with type-aware understanding (e.g., 'pkg.Foo(x)' -> 'x.Foo()').
//
// Parameters:
//   - ctx: execution context
//   - req: MCP tool request
//   - input: input data specifying the directory, find and replace patterns
//
// Returns:
//   - MCP tool call result
//   - AST rewrite result
//   - error if an error occurred while loading packages or parsing expressions
func ASTRewrite(ctx context.Context, req *mcp.CallToolRequest, input ASTRewriteInput) (
	*mcp.CallToolResult,
	ASTRewriteOutput,
	error,
) {
	start := logStart("ASTRewrite", map[string]string{
		"dir": input.Dir, "find": input.Find, "replace": input.Replace,
	})
	out := ASTRewriteOutput{
		ChangedFiles: []string{},
		Diffs:        []FileDiff{},
		TotalChanges: 0,
	}

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

	mode := packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedCompiledGoFiles

	pkgs, err := loadPackagesWithCache(ctx, input.Dir, mode)
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
					FromFile: "a/" + rel,
					ToFile:   "b/" + rel,
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

// ReadFunc returns the source code and metadata of a specific function or method.
//
// Parameters:
//   - ctx: execution context
//   - req: MCP tool request
//   - input: input data specifying the directory and function name (possibly with receiver)
//
// Returns:
//   - MCP tool call result
//   - function source code and its metadata
//   - error if the function is not found or an error occurred during analysis
func ReadFunc(ctx context.Context, req *mcp.CallToolRequest, input ReadFuncInput) (
	*mcp.CallToolResult,
	ReadFuncOutput,
	error,
) {
	start := logStart("ReadFunc", map[string]string{"dir": input.Dir, "name": input.Name})
	out := ReadFuncOutput{}

	defer func() { logEnd("ReadFunc", start, 1) }()

	mode := packages.NeedSyntax | packages.NeedCompiledGoFiles | packages.NeedTypes | packages.NeedTypesInfo

	pkgs, err := loadPackagesWithCache(ctx, input.Dir, mode)
	if err != nil {
		logError("ReadFunc", err, "failed to load packages")

		return fail(out, err)
	}

	target := input.Name

	var receiver, funcName string

	// Поддержка формата "Type.Method"
	if strings.Contains(target, ".") {
		parts := strings.SplitN(target, ".", 2)
		receiver, funcName = parts[0], parts[1]
	} else {
		funcName = target
	}

	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			fset := pkg.Fset
			abs := fset.File(file.Pos()).Name()
			rel, _ := filepath.Rel(input.Dir, abs)

			ast.Inspect(file, func(n ast.Node) bool {
				fd, ok := n.(*ast.FuncDecl)
				if !ok {
					return true
				}

				if fd.Name.Name != funcName {
					return true
				}

				recv := receiverName(fd)

				// Если указан получатель, фильтруем
				if receiver != "" && recv != receiver {
					return true
				}

				startPos := fset.Position(fd.Pos())
				endPos := fset.Position(fd.End())

				var buf bytes.Buffer

				err := format.Node(&buf, fset, fd)
				if err != nil {
					logError("ReadFunc", err, "failed to format function")

					return false
				}

				out.Function = FunctionSource{
					Name:       fd.Name.Name,
					Receiver:   recv,
					Package:    pkg.PkgPath,
					File:       rel,
					StartLine:  startPos.Line,
					EndLine:    endPos.Line,
					SourceCode: buf.String(),
				}

				return false // нашли — прерываем обход
			})

			if out.Function.Name != "" {
				return nil, out, nil
			}
		}
	}

	return nil, out, fmt.Errorf("function %q not found", input.Name)
}

// ReadFile returns information about a Go file: package, imports, symbols, line count, and (optionally) source code.
//
// Operation modes:
//   - "raw" — returns only source code and line count
//   - "summary" — returns package, imports, symbols, line count (without source)
//   - "ast" — full AST analysis, including source and symbols
func ReadFile(ctx context.Context, req *mcp.CallToolRequest, input ReadFileInput) (
	*mcp.CallToolResult,
	ReadFileOutput,
	error,
) {
	start := logStart("ReadFile", map[string]string{"dir": input.Dir, "file": input.File, "mode": input.Mode})
	out := ReadFileOutput{File: input.File}

	defer func() { logEnd("ReadFile", start, 1) }()

	// 1️⃣ Проверяем, что файл существует
	path := filepath.Join(input.Dir, input.File)

	content, err := os.ReadFile(path)
	if err != nil {
		logError("ReadFile", err, "failed to read file")

		return fail(out, fmt.Errorf("failed to read file %q: %w", input.File, err))
	}

	out.Source = string(content)
	out.LineCount = strings.Count(out.Source, "\n") + 1 // учитываем последнюю строку

	if input.Mode == "raw" {
		return nil, out, nil
	}

	// 2️⃣ Разбираем AST
	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		logError("ReadFile", err, "failed to parse file")

		return fail(out, fmt.Errorf("failed to parse file %q: %w", input.File, err))
	}

	out.Package = file.Name.Name

	// 3️⃣ Импорты
	for _, imp := range file.Imports {
		out.Imports = append(out.Imports, Import{
			Path: strings.Trim(imp.Path.Value, `"`),
			File: input.File,
			Line: fset.Position(imp.Pos()).Line,
		})
	}

	// 4️⃣ Символы
	ast.Inspect(file, func(n ast.Node) bool {
		switch decl := n.(type) {
		case *ast.FuncDecl:
			out.Symbols = append(out.Symbols, Symbol{
				Kind:     "func",
				Name:     decl.Name.Name,
				Package:  out.Package,
				File:     input.File,
				Line:     fset.Position(decl.Pos()).Line,
				Exported: decl.Name.IsExported(),
			})
		case *ast.TypeSpec:
			switch decl.Type.(type) {
			case *ast.StructType:
				out.Symbols = append(out.Symbols, Symbol{
					Kind:     "struct",
					Name:     decl.Name.Name,
					Package:  out.Package,
					File:     input.File,
					Line:     fset.Position(decl.Pos()).Line,
					Exported: decl.Name.IsExported(),
				})
			case *ast.InterfaceType:
				out.Symbols = append(out.Symbols, Symbol{
					Kind:     "interface",
					Name:     decl.Name.Name,
					Package:  out.Package,
					File:     input.File,
					Line:     fset.Position(decl.Pos()).Line,
					Exported: decl.Name.IsExported(),
				})
			default:
				out.Symbols = append(out.Symbols, Symbol{
					Kind:     "type",
					Name:     decl.Name.Name,
					Package:  out.Package,
					File:     input.File,
					Line:     fset.Position(decl.Pos()).Line,
					Exported: decl.Name.IsExported(),
				})
			}
		case *ast.GenDecl:
			switch decl.Tok {
			case token.CONST:
				for _, spec := range decl.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok {
						for _, n := range vs.Names {
							out.Symbols = append(out.Symbols, Symbol{
								Kind:     "const",
								Name:     n.Name,
								Package:  out.Package,
								File:     input.File,
								Line:     fset.Position(n.Pos()).Line,
								Exported: n.IsExported(),
							})
						}
					}
				}
			case token.VAR:
				for _, spec := range decl.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok {
						for _, n := range vs.Names {
							out.Symbols = append(out.Symbols, Symbol{
								Kind:     "var",
								Name:     n.Name,
								Package:  out.Package,
								File:     input.File,
								Line:     fset.Position(n.Pos()).Line,
								Exported: n.IsExported(),
							})
						}
					}
				}
			}
		}

		return true
	})

	// 5️⃣ Если режим summary — удаляем исходник, оставляем только метаданные
	if input.Mode == "summary" {
		out.Source = ""
	}

	return nil, out, nil
}

// ReadStruct returns a struct declaration with its fields, tags, comments, and optionally methods.
func ReadStruct(ctx context.Context, req *mcp.CallToolRequest, input ReadStructInput) (
	*mcp.CallToolResult,
	ReadStructOutput,
	error,
) {
	start := logStart("ReadStruct", map[string]string{"dir": input.Dir, "name": input.Name})
	out := ReadStructOutput{}

	defer func() { logEnd("ReadStruct", start, 1) }()

	mode := packages.NeedSyntax | packages.NeedTypes | packages.NeedCompiledGoFiles | packages.NeedTypesInfo

	pkgs, err := loadPackagesWithCache(ctx, input.Dir, mode)
	if err != nil {
		logError("ReadStruct", err, "failed to load packages")

		return fail(out, err)
	}

	target := input.Name

	var pkgName, structName string

	// Поддержка формата models.User
	if strings.Contains(target, ".") {
		parts := strings.SplitN(target, ".", 2)
		pkgName, structName = parts[0], parts[1]
	} else {
		structName = target
	}

	for _, pkg := range pkgs {
		if pkgName != "" && pkg.Name != pkgName {
			continue
		}

		for _, file := range pkg.Syntax {
			fset := pkg.Fset
			fileName := fset.File(file.Pos()).Name()
			relPath, _ := filepath.Rel(input.Dir, fileName)

			ast.Inspect(file, func(n ast.Node) bool {
				ts, ok := n.(*ast.TypeSpec)
				if !ok {
					return true
				}

				if ts.Name.Name != structName {
					return true
				}

				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					return true
				}

				var buf bytes.Buffer

				_ = format.Node(&buf, fset, ts)

				info := StructInfo{
					Name:     ts.Name.Name,
					Package:  pkg.PkgPath,
					File:     relPath,
					Line:     fset.Position(ts.Pos()).Line,
					Exported: ts.Name.IsExported(),
					Source:   buf.String(),
					Fields:   []StructField{},
					Doc:      "",
					Methods:  []string{},
				}

				// Doc-комментарий к структуре
				if ts.Doc != nil {
					info.Doc = strings.TrimSpace(ts.Doc.Text())
				}

				// Поля структуры
				for _, field := range st.Fields.List {
					fieldType := exprString(field.Type)

					tag := ""
					if field.Tag != nil {
						tag = strings.Trim(field.Tag.Value, "`")
					}

					doc := ""
					if field.Doc != nil {
						doc = strings.TrimSpace(field.Doc.Text())
					}

					for _, name := range field.Names {
						info.Fields = append(info.Fields, StructField{
							Name: name.Name,
							Type: fieldType,
							Tag:  tag,
							Doc:  doc,
						})
					}

					// анонимные (embedded) поля
					if len(field.Names) == 0 {
						info.Fields = append(info.Fields, StructField{
							Name: fieldType,
							Type: fieldType,
							Tag:  tag,
							Doc:  doc,
						})
					}
				}

				// Методы
				if input.IncludeMethods {
					for _, f := range pkg.Syntax {
						ast.Inspect(f, func(n ast.Node) bool {
							fd, ok := n.(*ast.FuncDecl)
							if !ok || fd.Recv == nil {
								return true
							}

							if receiverName(fd) == structName {
								info.Methods = append(info.Methods, fd.Name.Name)
							}

							return true
						})
					}

					sort.Strings(info.Methods)
				}

				out.Struct = info

				return false // нашли нужную структуру
			})

			if out.Struct.Name != "" {
				return nil, out, nil
			}
		}
	}

	return nil, out, fmt.Errorf("struct %q not found", input.Name)
}
