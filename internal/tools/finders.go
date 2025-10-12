package tools

import (
	"context"
	"fmt"
	"go/ast"
	"go/types"
	"sort"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/go/packages"
)

const (
	defaultBestContextUsages       = 3
	defaultBestContextTests        = 2
	defaultBestContextDependencies = 5
	maxDependencySourceFiles       = 3
)

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
func FindReferences(ctx context.Context, _ *mcp.CallToolRequest, input FindReferencesInput) (
	*mcp.CallToolResult,
	FindReferencesOutput,
	error,
) {
	out := FindReferencesOutput{}
	if err := validatePagination(input.Limit, input.Offset); err != nil {
		return fail(out, err)
	}

	start := logStart("FindReferences", logFields(
		input.Dir,
		newLogField("ident", input.Ident),
		newLogField("kind", input.Kind),
	))

	resultCount := 0

	defer func() { logEnd("FindReferences", start, resultCount) }()

	mode := loadModeSyntaxTypes | packages.NeedFiles

	pkgs, err := loadPackagesWithCacheIncludeTests(ctx, input.Dir, mode)
	if err != nil {
		return fail(out, err)
	}

	target := findTargetObject(ctx, pkgs, input.Ident, input.Kind)
	if target == nil {
		return nil, out, fmt.Errorf("symbol %q not found", input.Ident)
	}

	records := make([]locationRecord, 0)

	for _, pkg := range pkgs {
		if shouldStop(ctx) {
			return fail(out, context.Canceled)
		}

		for i, file := range pkg.Syntax {
			relPath := resolveFilePath(pkg, input.Dir, i, file)
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
				appendReference(&records, input.Dir, relPath, pos.Line, snip)

				return true
			})
		}
	}

	sortLocationRecords(records)

	out.Total = len(records)

	offset, paged := applyPagination(records, input.Offset, input.Limit)
	out.Offset = offset
	out.Limit = input.Limit

	resultCount = len(paged)
	out.Groups = makeReferenceGroups(paged)

	return nil, out, nil
}

// FindBestContext returns a curated, minimal context bundle for a symbol.
//
// The bundle includes:
//   - Primary definition (and any additional definitions)
//   - Key non-test usages (defaults to 3)
//   - Key test usages (defaults to 2)
//   - Direct imports from the definition files (defaults to 5)
func FindBestContext(ctx context.Context, _ *mcp.CallToolRequest, input FindBestContextInput) (
	*mcp.CallToolResult,
	FindBestContextOutput,
	error,
) {
	out := FindBestContextOutput{Symbol: input.Ident}

	start := logStart("FindBestContext", logFields(
		input.Dir,
		newLogField("ident", input.Ident),
		newLogField("kind", input.Kind),
	))

	resultCount := 0

	defer func() { logEnd("FindBestContext", start, resultCount) }()

	mode := loadModeSyntaxTypes | packages.NeedFiles

	pkgs, err := loadPackagesWithCacheIncludeTests(ctx, input.Dir, mode)
	if err != nil {
		return fail(out, err)
	}

	target := findTargetObject(ctx, pkgs, input.Ident, input.Kind)
	if target == nil {
		return nil, out, fmt.Errorf("symbol %q not found", input.Ident)
	}

	out.Kind = objStringKind(target)

	maxUsages := input.MaxUsages
	if maxUsages <= 0 {
		maxUsages = defaultBestContextUsages
	}

	maxTestUsages := input.MaxTestUsages
	if maxTestUsages <= 0 {
		maxTestUsages = defaultBestContextTests
	}

	maxDependencies := input.MaxDependencies
	if maxDependencies <= 0 {
		maxDependencies = defaultBestContextDependencies
	}

	definitionRecords := make([]locationRecord, 0)
	usageRecords := make([]locationRecord, 0)
	testRecords := make([]locationRecord, 0)

	seenDefinitions := make(map[string]struct{})
	seenUsages := make(map[string]struct{})
	seenTests := make(map[string]struct{})

	definitionFiles := make(map[string]struct{})
	fileImports := make(map[string][]string)

	err = walkPackageFiles(ctx, pkgs, input.Dir, func(pkg *packages.Package, file *ast.File, relPath string, fileIndex int) error {
		if relPath == "" {
			return nil
		}

		lines := getFileLines(pkg.Fset, file)
		fileImports[relPath] = collectUniqueImports(file)

		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.SelectorExpr:
				selIdent := node.Sel
				if selIdent == nil || selIdent.Name != input.Ident {
					return true
				}

				obj := selectorObject(pkg.TypesInfo, node)
				if !matchesTargetObject(obj, target) {
					return true
				}

				pos := pkg.Fset.Position(selIdent.Pos())
				if pos.Filename == "" {
					return true
				}

				rec := locationRecord{
					File:    relPath,
					Line:    pos.Line,
					Snippet: extractSnippet(lines, pos.Line),
				}
				key := fmt.Sprintf("%s:%d", relPath, pos.Line)

				if strings.HasSuffix(relPath, "_test.go") {
					if _, ok := seenTests[key]; !ok {
						testRecords = append(testRecords, rec)
						seenTests[key] = struct{}{}
					}
				} else {
					if _, ok := seenUsages[key]; !ok {
						usageRecords = append(usageRecords, rec)
						seenUsages[key] = struct{}{}
					}
				}

				return true
			case *ast.Ident:
				ident := node
				if ident.Name != input.Ident {
					return true
				}

				obj := objectForIdent(pkg.TypesInfo, ident)
				if !matchesTargetObject(obj, target) {
					return true
				}

				pos := pkg.Fset.Position(ident.Pos())
				if pos.Filename == "" {
					return true
				}

				rec := locationRecord{
					File:    relPath,
					Line:    pos.Line,
					Snippet: extractSnippet(lines, pos.Line),
				}
				key := fmt.Sprintf("%s:%d", relPath, pos.Line)

				if defObj := pkg.TypesInfo.Defs[ident]; defObj != nil && matchesTargetObject(defObj, target) {
					if _, ok := seenDefinitions[key]; !ok {
						definitionRecords = append(definitionRecords, rec)
						seenDefinitions[key] = struct{}{}
						definitionFiles[relPath] = struct{}{}
					}

					return true
				}

				if strings.HasSuffix(relPath, "_test.go") {
					if _, ok := seenTests[key]; !ok {
						testRecords = append(testRecords, rec)
						seenTests[key] = struct{}{}
					}
				} else {
					if _, ok := seenUsages[key]; !ok {
						usageRecords = append(usageRecords, rec)
						seenUsages[key] = struct{}{}
					}
				}

				return true
			default:
				return true
			}
		})

		return nil
	})
	if err != nil {
		return fail(out, err)
	}

	sortLocationRecords(definitionRecords)
	sortLocationRecords(usageRecords)
	sortLocationRecords(testRecords)

	// Ensure any entries from usageRecords that reside in test files are categorised as tests.
	if len(usageRecords) > 0 {
		filteredUsages := make([]locationRecord, 0, len(usageRecords))

		for _, rec := range usageRecords {
			if strings.HasSuffix(rec.File, "_test.go") {
				testRecords = append(testRecords, rec)

				continue
			}

			filteredUsages = append(filteredUsages, rec)
		}

		usageRecords = filteredUsages

		sortLocationRecords(testRecords)
	}

	defLocations := toContextLocations(definitionRecords, 0)
	if len(defLocations) == 0 {
		return nil, out, fmt.Errorf("definition for %q not found", input.Ident)
	}

	out.Definition = &defLocations[0]
	if len(defLocations) > 1 {
		out.AdditionalDefinitions = append(out.AdditionalDefinitions, defLocations[1:]...)
	}

	out.KeyUsages = toContextLocations(usageRecords, maxUsages)
	out.TestUsages = toContextLocations(testRecords, maxTestUsages)
	out.Dependencies = buildContextDependencies(definitionFiles, fileImports, maxDependencies)

	resultCount = len(definitionRecords) + len(out.KeyUsages) + len(out.TestUsages)

	return nil, out, nil
}

func toContextLocations(records []locationRecord, limit int) []ContextLocation {
	if len(records) == 0 {
		return nil
	}

	slice := records
	if limit > 0 && len(slice) > limit {
		slice = slice[:limit]
	}

	result := make([]ContextLocation, 0, len(slice))

	for _, rec := range slice {
		result = append(result, ContextLocation(rec))
	}

	return result
}

func collectUniqueImports(file *ast.File) []string {
	if file == nil || len(file.Imports) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(file.Imports))
	imports := make([]string, 0, len(file.Imports))

	for _, spec := range file.Imports {
		if spec == nil || spec.Path == nil {
			continue
		}

		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			path = strings.Trim(spec.Path.Value, "`\"")
		}

		if path == "" {
			continue
		}

		if _, ok := seen[path]; ok {
			continue
		}

		seen[path] = struct{}{}
		imports = append(imports, path)
	}

	sort.Strings(imports)

	return imports
}

func buildContextDependencies(definitionFiles map[string]struct{}, fileImports map[string][]string, maxDeps int) []ContextDependency {
	if len(definitionFiles) == 0 {
		return nil
	}

	depFiles := make(map[string]map[string]struct{})

	for file := range definitionFiles {
		imports := fileImports[file]
		if len(imports) == 0 {
			continue
		}

		for _, imp := range imports {
			if depFiles[imp] == nil {
				depFiles[imp] = make(map[string]struct{})
			}

			depFiles[imp][file] = struct{}{}
		}
	}

	if len(depFiles) == 0 {
		return nil
	}

	depNames := make([]string, 0, len(depFiles))

	for imp := range depFiles {
		depNames = append(depNames, imp)
	}

	sort.Strings(depNames)

	if maxDeps > 0 && len(depNames) > maxDeps {
		depNames = depNames[:maxDeps]
	}

	result := make([]ContextDependency, 0, len(depNames))

	for _, dep := range depNames {
		filesSet := depFiles[dep]
		files := make([]string, 0, len(filesSet))

		for file := range filesSet {
			files = append(files, file)
		}

		sort.Strings(files)

		if len(files) > maxDependencySourceFiles {
			files = files[:maxDependencySourceFiles]
		}

		result = append(result, ContextDependency{
			Import:      dep,
			SourceFiles: files,
		})
	}

	return result
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
func FindDefinitions(ctx context.Context, _ *mcp.CallToolRequest, input FindDefinitionsInput) (
	*mcp.CallToolResult,
	FindDefinitionsOutput,
	error,
) {
	out := FindDefinitionsOutput{}
	if err := validatePagination(input.Limit, input.Offset); err != nil {
		return fail(out, err)
	}

	start := logStart("FindDefinitions", logFields(
		input.Dir,
		newLogField("ident", input.Ident),
		newLogField("kind", input.Kind),
	))

	resultCount := 0

	defer func() { logEnd("FindDefinitions", start, resultCount) }()

	mode := loadModeSyntaxTypes

	pkgs, err := loadPackagesWithCache(ctx, input.Dir, mode)
	if err != nil {
		return fail(out, err)
	}

	records := make([]locationRecord, 0)

	for _, pkg := range pkgs {
		if shouldStop(ctx) {
			return fail(out, context.Canceled)
		}

		if obj := findTargetObject(ctx, []*packages.Package{pkg}, input.Ident, input.Kind); obj != nil {
			appendDefinition(&records, input.Dir, pkg.Fset, obj.Pos(), input.File)
		}
	}

	sortLocationRecords(records)

	out.Total = len(records)

	offset, paged := applyPagination(records, input.Offset, input.Limit)
	out.Offset = offset
	out.Limit = input.Limit

	resultCount = len(paged)
	out.Groups = makeDefinitionGroups(paged)

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
func FindImplementations(ctx context.Context, _ *mcp.CallToolRequest, input FindImplementationsInput) (
	*mcp.CallToolResult,
	FindImplementationsOutput,
	error,
) {
	start := logStart("FindImplementations", logFields(
		input.Dir,
		newLogField("name", input.Name),
	))
	out := FindImplementationsOutput{Implementations: []Implementation{}}

	defer func() { logEnd("FindImplementations", start, len(out.Implementations)) }()

	mode := loadModeSyntaxTypes

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
	for i, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			relPath := resolveFilePath(pkg, input.Dir, i, file)

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

func objectForIdent(info *types.Info, ident *ast.Ident) types.Object {
	if info == nil || ident == nil {
		return nil
	}

	if obj := info.ObjectOf(ident); obj != nil {
		return obj
	}

	if obj := info.Uses[ident]; obj != nil {
		return obj
	}

	for sel, selection := range info.Selections {
		if sel != nil && sel.Sel == ident && selection != nil {
			return selection.Obj()
		}
	}

	return nil
}

func selectorObject(info *types.Info, sel *ast.SelectorExpr) types.Object {
	if info == nil || sel == nil {
		return nil
	}

	if selection, ok := info.Selections[sel]; ok && selection != nil {
		return selection.Obj()
	}

	return objectForIdent(info, sel.Sel)
}

func matchesTargetObject(obj types.Object, target types.Object) bool {
	if obj == nil || target == nil {
		return false
	}

	if sameObject(obj, target) {
		return true
	}

	if obj.Name() != target.Name() {
		return false
	}

	if types.Identical(obj.Type(), target.Type()) {
		return true
	}

	if objPkg, targetPkg := obj.Pkg(), target.Pkg(); objPkg != nil && targetPkg != nil && objPkg.Path() == targetPkg.Path() {
		if types.AssignableTo(obj.Type(), target.Type()) && types.AssignableTo(target.Type(), obj.Type()) {
			return true
		}
	}

	if objFunc, ok := obj.(*types.Func); ok {
		if targetFunc, ok2 := target.(*types.Func); ok2 {
			if objFunc.FullName() == targetFunc.FullName() {
				return true
			}
		}
	}

	if types.ObjectString(obj, nil) == types.ObjectString(target, nil) {
		return true
	}

	return false
}
