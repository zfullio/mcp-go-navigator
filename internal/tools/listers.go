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
	start := logStart("ListPackages", logFields(input.Dir))
	out := ListPackagesOutput{Packages: []string{}}

	defer func() { logEnd("ListPackages", start, len(out.Packages)) }()

	mode := loadModeBasic

	pkgs, err := loadPackagesWithCache(ctx, input.Dir, mode)
	if err != nil {
		logError("ListPackages", err, "failed to load packages")

		return fail(out, err)
	}

	for _, pkg := range pkgs {
		path := normalizePackagePath(pkg)
		if path == "" && pkg.Name != "" {
			path = pkg.Name
		}

		out.Packages = append(out.Packages, path)
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
	start := logStart("ListSymbols", logFields(
		input.Dir,
		newLogField("package", input.Package),
	))
	symbols := []Symbol{}

	defer func() { logEnd("ListSymbols", start, len(symbols)) }()

	mode := loadModeSyntaxTypesNamedFiles

	_, filteredPkgs, err := loadFilteredPackages(ctx, input.Dir, mode, input.Package, "ListSymbols")
	if err != nil {
		return fail(ListSymbolsOutput{}, err)
	}

	if err := walkPackageFiles(ctx, filteredPkgs, input.Dir, func(pkg *packages.Package, file *ast.File, relPath string, _ int) error {
		pkgPath := normalizePackagePath(pkg)
		if pkgPath == "" && file.Name != nil {
			pkgPath = file.Name.Name
		}

		if pkgPath == "" {
			pkgPath = "(unknown)"
		}

		for _, sym := range collectSymbols(file, pkg.Fset, pkgPath, relPath) {
			switch sym.Kind {
			case "func", "struct", "interface", "method":
				symbols = append(symbols, sym)
			}
		}

		return nil
	}); err != nil {
		return fail(ListSymbolsOutput{}, err)
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
	start := logStart("ListImports", logFields(
		input.Dir,
		newLogField("package", input.Package),
	))
	out := ListImportsOutput{}

	defer func() { logEnd("ListImports", start, len(out.Imports)) }()

	mode := loadModeBasicSyntax

	flatImports := make([]Import, 0)

	_, filteredPkgs, err := loadFilteredPackages(ctx, input.Dir, mode, input.Package, "ListImports")
	if err != nil {
		return fail(out, err)
	}

	if err := walkPackageFiles(ctx, filteredPkgs, input.Dir, func(pkg *packages.Package, file *ast.File, relPath string, _ int) error {
		for _, imp := range file.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			pos := pkg.Fset.Position(imp.Pos())
			flatImports = append(flatImports, Import{Path: path, File: relPath, Line: pos.Line})
		}

		return nil
	}); err != nil {
		return fail(out, err)
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
	start := logStart("ListInterfaces", logFields(
		input.Dir,
		newLogField("package", input.Package),
	))
	out := ListInterfacesOutput{}

	defer func() { logEnd("ListInterfaces", start, len(out.Interfaces)) }()

	mode := loadModeBasicSyntax

	interfacesByPackage := make(map[string][]InterfaceInfo)

	_, filteredPkgs, err := loadFilteredPackages(ctx, input.Dir, mode, input.Package, "ListInterfaces")
	if err != nil {
		return fail(out, err)
	}

	if err := walkPackageFiles(ctx, filteredPkgs, input.Dir, func(pkg *packages.Package, file *ast.File, relPath string, _ int) error {
		pkgKey := normalizePackagePath(pkg)
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

		return nil
	}); err != nil {
		return fail(out, err)
	}

	out.Interfaces = groupInterfacesByPackage(interfacesByPackage)

	return nil, out, nil
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
	start := logStart("ProjectSchema", logFields(
		input.Dir,
		newLogField("depth", input.Depth),
	))
	out := ProjectSchemaOutput{}

	defer func() { logEnd("ProjectSchema", start, len(out.Packages)) }()

	// Determine depth level
	depth := input.Depth
	if depth == "" {
		depth = "standard" // default level
	}

	// Adjust analysis mode based on depth
	mode := loadModeBasic
	if depth == "standard" || depth == "deep" {
		mode |= packages.NeedImports | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo
	} else {
		mode |= packages.NeedImports // minimal for summary
	}

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

	var structCount, funcCount, ifaceCount int

	pkgMap := map[string]ProjectPackage{}
	depGraph := map[string][]string{}
	externalDeps := map[string]struct{}{}

	var allInterfaces []ProjectInterface // Only initialize if needed for standard/deep mode

	for _, pkg := range pkgs {
		pkgPath := normalizePackagePath(pkg)
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

		// Only analyze AST if we need detailed information
		if depth == "standard" || depth == "deep" {
			pkgInterfaceMethods := make(map[string][]string)
			interfaceListed := make(map[string]struct{})

			for i, file := range pkg.Syntax {
				relPath := resolveFilePath(pkg, input.Dir, i, file)

				for _, sym := range collectSymbols(file, pkg.Fset, pkgPath, relPath) {
					switch sym.Kind {
					case "struct":
						symbols.Structs = append(symbols.Structs, sym.Name)
						structCount++
					case "interface":
						if _, listed := interfaceListed[sym.Name]; !listed {
							symbols.Interfaces = append(symbols.Interfaces, sym.Name)
							ifaceCount++
							interfaceListed[sym.Name] = struct{}{}
						}
					case "type":
						symbols.Types = append(symbols.Types, sym.Name)
					case "func":
						symbols.Functions = append(symbols.Functions, sym.Name)
						funcCount++
					case "method":
						parts := strings.SplitN(sym.Name, ".", 2)
						if len(parts) == 2 {
							pkgInterfaceMethods[parts[0]] = append(pkgInterfaceMethods[parts[0]], parts[1])
						}
					}
				}
			}

			for _, ifaceName := range symbols.Interfaces {
				methods := pkgInterfaceMethods[ifaceName]
				allInterfaces = append(allInterfaces, ProjectInterface{
					Name: ifaceName, DefinedIn: pkgPath, Methods: methods,
				})
			}
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

	// Only include interfaces if we did detailed analysis
	if depth == "standard" || depth == "deep" {
		out.Interfaces = allInterfaces
	}

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
