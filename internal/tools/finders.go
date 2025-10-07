package tools

import (
	"context"
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/go/packages"
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
func FindReferences(ctx context.Context, req *mcp.CallToolRequest, input FindReferencesInput) (
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

	pkgs, err := loadPackagesWithCache(ctx, input.Dir, mode)
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
func FindImplementations(ctx context.Context, req *mcp.CallToolRequest, input FindImplementationsInput) (
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
