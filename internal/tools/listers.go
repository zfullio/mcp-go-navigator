package tools

import (
	"context"
	"go/ast"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog/log"
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

	mode := packages.NeedSyntax | packages.NeedTypes | packages.NeedCompiledGoFiles | packages.NeedTypesInfo | packages.NeedName | packages.NeedFiles

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

		for i, file := range pkg.Syntax {
			relPath := resolveFilePath(pkg, input.Dir, i, file)

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
		for i, file := range pkg.Syntax {
			relPath := resolveFilePath(pkg, input.Dir, i, file)
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

	mode := packages.NeedSyntax | packages.NeedCompiledGoFiles | packages.NeedName

	pkgs, err := loadPackagesWithCache(ctx, input.Dir, mode)
	if err != nil {
		logError("ListInterfaces", err, "failed to load packages")

		return fail(out, err)
	}

	interfacesByPackage := make(map[string][]InterfaceInfo)

	for _, pkg := range pkgs {
		for i, file := range pkg.Syntax {
			relPath := resolveFilePath(pkg, input.Dir, i, file)

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

func resolveFilePath(pkg *packages.Package, inputDir string, fileIndex int, file *ast.File) string {
	var absPath string

	// 1. Пытаемся получить путь через FileSet
	if f := pkg.Fset.File(file.Pos()); f != nil {
		absPath = f.Name()
	}

	// 2. Если не удалось — fallback через CompiledGoFiles и GoFiles
	if absPath == "" {
		if len(pkg.CompiledGoFiles) > fileIndex {
			absPath = pkg.CompiledGoFiles[fileIndex]
		} else if len(pkg.GoFiles) > fileIndex {
			absPath = pkg.GoFiles[fileIndex]
		}
	}

	// 3. Делаем путь относительным к input.Dir (для удобства)
	if absPath == "" {
		return ""
	}

	relPath, err := filepath.Rel(inputDir, absPath)
	if err != nil {
		return absPath // на случай ошибки просто вернуть абсолютный
	}

	return relPath
}

// ProjectSchema aggregates full structural metadata of a Go module,
// including packages, symbols, interfaces, imports, and dependency graph.
//
// Parameters:
//   - ctx: execution context
//   - req: MCP tool request
//   - input: input data specifying the module directory and depth level
//
// Returns:
//   - MCP tool call result
//   - project schema with detailed structure and dependencies
//   - error if packages failed to load or parsing encountered issues
func ProjectSchema(ctx context.Context, req *mcp.CallToolRequest, input ProjectSchemaInput) (
	*mcp.CallToolResult,
	ProjectSchemaOutput,
	error,
) {
	start := logStart("ProjectSchema", map[string]string{
		"dir":   input.Dir,
		"depth": input.Depth,
	})
	out := ProjectSchemaOutput{}
	defer func() { logEnd("ProjectSchema", start, len(out.Packages)) }()

	mode := packages.NeedName |
		packages.NeedCompiledGoFiles |
		packages.NeedImports |
		packages.NeedSyntax |
		packages.NeedTypes |
		packages.NeedTypesInfo

	pkgs, err := loadPackagesWithCache(ctx, input.Dir, mode)
	if err != nil {
		logError("ProjectSchema", err, "failed to load packages")
		return fail(out, err)
	}

	// Read go.mod metadata
	moduleName, goVersion := readGoModInfo(input.Dir)
	out.Module = moduleName
	out.GoVersion = goVersion
	out.RootDir = input.Dir

	var (
		structCount, funcCount, ifaceCount int
	)

	pkgMap := map[string]ProjectPackage{}
	depGraph := map[string][]string{}
	externalDeps := map[string]struct{}{}
	allInterfaces := []ProjectInterface{}

	for _, pkg := range pkgs {
		pkgPath := pkg.PkgPath
		if pkgPath == "" {
			pkgPath = pkg.Name
		}

		symbols := ProjectPackageSymbols{}
		imports := make([]string, 0, len(pkg.Imports))

		for imp := range pkg.Imports {
			imports = append(imports, imp)
			depGraph[pkgPath] = append(depGraph[pkgPath], imp)

			if !strings.HasPrefix(imp, moduleName) &&
				!strings.HasPrefix(imp, "std/") &&
				!strings.Contains(imp, "/internal/") {
				externalDeps[imp] = struct{}{}
			}
		}

		for i, file := range pkg.Syntax {
			_ = i // keep consistent with your style, if relPath needed later
			ast.Inspect(file, func(n ast.Node) bool {
				switch ts := n.(type) {
				case *ast.TypeSpec:
					switch t := ts.Type.(type) {
					case *ast.StructType:
						symbols.Structs = append(symbols.Structs, ts.Name.Name)
						structCount++
					case *ast.InterfaceType:
						methods := []string{}
						if t.Methods != nil {
							for _, m := range t.Methods.List {
								if len(m.Names) > 0 {
									methods = append(methods, m.Names[0].Name)
								}
							}
						}
						allInterfaces = append(allInterfaces, ProjectInterface{
							Name: ts.Name.Name, DefinedIn: pkgPath, Methods: methods,
						})
						symbols.Interfaces = append(symbols.Interfaces, ts.Name.Name)
						ifaceCount++
					default:
						symbols.Types = append(symbols.Types, ts.Name.Name)
					}
				case *ast.FuncDecl:
					symbols.Functions = append(symbols.Functions, ts.Name.Name)
					funcCount++
				}
				return true
			})
		}

		pkgMap[pkgPath] = ProjectPackage{
			Path:    pkgPath,
			Name:    pkg.Name,
			Imports: imports,
			Symbols: symbols,
		}
	}

	// Collect sorted results
	for _, pkg := range pkgMap {
		out.Packages = append(out.Packages, pkg)
	}
	sort.Slice(out.Packages, func(i, j int) bool { return out.Packages[i].Path < out.Packages[j].Path })

	for dep := range externalDeps {
		out.ExternalDeps = append(out.ExternalDeps, dep)
	}
	sort.Strings(out.ExternalDeps)

	out.DependencyGraph = depGraph
	out.Interfaces = allInterfaces
	out.Summary = ProjectSummary{
		PackageCount:   len(out.Packages),
		FunctionCount:  funcCount,
		StructCount:    structCount,
		InterfaceCount: ifaceCount,
	}

	return nil, out, nil
}

// readGoModInfo reads the module name and Go version from go.mod located in the given directory.
//
// Returns:
//   - moduleName: value after "module" directive
//   - goVersion: value after "go" directive
func readGoModInfo(dir string) (moduleName, goVersion string) {
	modFile := filepath.Join(dir, "go.mod")

	data, err := os.ReadFile(modFile)
	if err != nil {
		log.Debug().Err(err).Str("file", modFile).Msg("go.mod not found or unreadable")
		return "", ""
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			moduleName = strings.TrimSpace(strings.TrimPrefix(line, "module"))
			continue
		}
		if strings.HasPrefix(line, "go ") {
			goVersion = strings.TrimSpace(strings.TrimPrefix(line, "go"))
		}
	}

	if moduleName == "" {
		log.Warn().Str("dir", dir).Msg("module name not found in go.mod")
	}
	if goVersion == "" {
		log.Debug().Str("dir", dir).Msg("Go version not found in go.mod")
	}

	return moduleName, goVersion
}
