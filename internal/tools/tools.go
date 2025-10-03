package tools

import (
	"context"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/go/packages"
)

// packageCache stores loaded packages by directory to avoid redundant parsing
var packageCache = struct {
	sync.RWMutex
	pkgs map[string][]*packages.Package
}{
	pkgs: make(map[string][]*packages.Package),
}

type ListPackagesInput struct {
	Dir string `json:"dir" jsonschema:"directory to scan for packages"`
}

type ListPackagesOutput struct {
	Packages []string `json:"packages" jsonschema:"list of package paths"`
}

type ListSymbolsInput struct {
	Dir     string `json:"dir"     jsonschema:"directory to scan for packages"`
	Package string `json:"package" jsonschema:"package path to inspect"`
}

type Symbol struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	File string `json:"file"`
	Line int    `json:"line"`
}

type ListSymbolsOutput struct {
	Symbols []Symbol `json:"symbols"`
}

type FindReferencesInput struct {
	Dir   string `json:"dir"   jsonschema:"directory to scan"`
	Ident string `json:"ident" jsonschema:"identifier to search for"`
}

type Reference struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Snippet string `json:"snippet"`
}

type FindReferencesOutput struct {
	References []Reference `json:"references"`
}

type FindDefinitionsInput struct {
	Dir   string `json:"dir"   jsonschema:"directory to scan"`
	Ident string `json:"ident" jsonschema:"identifier to search for definition"`
}

type Definition struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Snippet string `json:"snippet"`
}

type FindDefinitionsOutput struct {
	Definitions []Definition `json:"definitions"`
}

type RenameSymbolInput struct {
	Dir     string `json:"dir"     jsonschema:"directory to scan"`
	OldName string `json:"oldName" jsonschema:"symbol name to rename"`
	NewName string `json:"newName" jsonschema:"new symbol name"`
}

type RenameSymbolOutput struct {
	ChangedFiles []string `json:"changedFiles"`
}

type ListImportsInput struct {
	Dir string `json:"dir" jsonschema:"directory to scan for Go files"`
}

type Import struct {
	Path string `json:"path"`
	File string `json:"file"`
	Line int    `json:"line"`
}

type ListImportsOutput struct {
	Imports []Import `json:"imports"`
}

type ListInterfacesInput struct {
	Dir string `json:"dir" jsonschema:"directory to scan for Go files"`
}

type InterfaceMethod struct {
	Name string `json:"name"`
	Line int    `json:"line"`
}

type InterfaceInfo struct {
	Name    string            `json:"name"`
	File    string            `json:"file"`
	Line    int               `json:"line"`
	Methods []InterfaceMethod `json:"methods"`
}

type ListInterfacesOutput struct {
	Interfaces []InterfaceInfo `json:"interfaces"`
}

func ListPackages(ctx context.Context, req *mcp.CallToolRequest, input ListPackagesInput) (
	*mcp.CallToolResult,
	ListPackagesOutput,
	error,
) {
	cfg := &packages.Config{
		Mode:    packages.NeedName | packages.NeedFiles,
		Dir:     input.Dir,
		Context: ctx,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, ListPackagesOutput{}, err
	}

	out := ListPackagesOutput{Packages: []string{}}
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
	mode := packages.NeedSyntax | packages.NeedTypes | packages.NeedFiles | packages.NeedTypesInfo
	pkgs, err := loadPackagesWithCache(ctx, input.Dir, mode)
	if err != nil {
		return nil, ListSymbolsOutput{}, err
	}

	out := ListSymbolsOutput{Symbols: []Symbol{}}

	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			fname := pkg.Fset.File(file.Pos()).Name()
			ast.Inspect(file, func(n ast.Node) bool {
				switch decl := n.(type) {
				case *ast.FuncDecl:
					out.Symbols = append(out.Symbols, Symbol{
						Kind: "func",
						Name: decl.Name.Name,
						File: fname,
						Line: pkg.Fset.Position(decl.Pos()).Line,
					})
				case *ast.TypeSpec:
					switch t := decl.Type.(type) {
					case *ast.StructType:
						out.Symbols = append(out.Symbols, Symbol{
							Kind: "struct",
							Name: decl.Name.Name,
							File: fname,
							Line: pkg.Fset.Position(decl.Pos()).Line,
						})
					case *ast.InterfaceType:
						out.Symbols = append(out.Symbols, Symbol{
							Kind: "interface",
							Name: decl.Name.Name,
							File: fname,
							Line: pkg.Fset.Position(decl.Pos()).Line,
						})
						// можно дополнительно перечислять методы интерфейса:
						for _, m := range t.Methods.List {
							if len(m.Names) > 0 {
								out.Symbols = append(out.Symbols, Symbol{
									Kind: "method",
									Name: decl.Name.Name + "." + m.Names[0].Name,
									File: fname,
									Line: pkg.Fset.Position(m.Pos()).Line,
								})
							}
						}
					default:
						// другие типы (alias, enum) можно добавить при необходимости
					}
				}

				return true
			})
		}
	}

	return nil, out, nil
}

func FindReferences(ctx context.Context, req *mcp.CallToolRequest, input FindReferencesInput) (
	*mcp.CallToolResult,
	FindReferencesOutput,
	error,
) {
	mode := packages.NeedSyntax | packages.NeedFiles | packages.NeedTypesInfo
	pkgs, err := loadPackagesWithCache(ctx, input.Dir, mode)
	if err != nil {
		return nil, FindReferencesOutput{}, err
	}

	out := FindReferencesOutput{References: []Reference{}}

	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			fname := pkg.Fset.File(file.Pos()).Name()
			lines := getFileLines(pkg.Fset, file)

			ast.Inspect(file, func(n ast.Node) bool {
				if ident, ok := n.(*ast.Ident); ok && ident.Name == input.Ident {
					pos := pkg.Fset.Position(ident.Pos())

					snip := ""
					if pos.Line-1 < len(lines) {
						snip = strings.TrimSpace(lines[pos.Line-1])
					}

					out.References = append(out.References, Reference{
						File:    fname,
						Line:    pos.Line,
						Snippet: snip,
					})
				}

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
	mode := packages.NeedSyntax | packages.NeedFiles | packages.NeedTypesInfo
	pkgs, err := loadPackagesWithCache(ctx, input.Dir, mode)
	if err != nil {
		return nil, FindDefinitionsOutput{}, err
	}

	out := FindDefinitionsOutput{Definitions: []Definition{}}

	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			fname := pkg.Fset.File(file.Pos()).Name()
			lines := getFileLines(pkg.Fset, file)

			ast.Inspect(file, func(n ast.Node) bool {
				switch decl := n.(type) {
				case *ast.TypeSpec:
					if decl.Name.Name == input.Ident {
						pos := pkg.Fset.Position(decl.Pos())

						snip := ""
						if pos.Line-1 < len(lines) {
							snip = strings.TrimSpace(lines[pos.Line-1])
						}

						out.Definitions = append(out.Definitions, Definition{
							File:    fname,
							Line:    pos.Line,
							Snippet: snip,
						})
					}
				case *ast.FuncDecl:
					if decl.Name.Name == input.Ident {
						pos := pkg.Fset.Position(decl.Pos())

						snip := ""
						if pos.Line-1 < len(lines) {
							snip = strings.TrimSpace(lines[pos.Line-1])
						}

						out.Definitions = append(out.Definitions, Definition{
							File:    fname,
							Line:    pos.Line,
							Snippet: snip,
						})
					}
				case *ast.ValueSpec:
					for _, name := range decl.Names {
						if name.Name == input.Ident {
							pos := pkg.Fset.Position(decl.Pos())

							snip := ""
							if pos.Line-1 < len(lines) {
								snip = strings.TrimSpace(lines[pos.Line-1])
							}

							out.Definitions = append(out.Definitions, Definition{
								File:    fname,
								Line:    pos.Line,
								Snippet: snip,
							})
						}
					}
				}

				return true
			})
		}
	}

	return nil, out, nil
}

func RenameSymbol(ctx context.Context, req *mcp.CallToolRequest, input RenameSymbolInput) (
	*mcp.CallToolResult,
	RenameSymbolOutput,
	error,
) {
	out := RenameSymbolOutput{ChangedFiles: []string{}}

	err := filepath.Walk(input.Dir, func(path string, info os.FileInfo, err error) error {
		// Check context cancellation periodically
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			return err
		}

		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		fset := token.NewFileSet()

		node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}

		changed := false

		ast.Inspect(node, func(n ast.Node) bool {
			// Check context cancellation during AST traversal
			select {
			case <-ctx.Done():
				return false
			default:
			}

			if ident, ok := n.(*ast.Ident); ok {
				if ident.Name == input.OldName {
					ident.Name = input.NewName
					changed = true
				}
			}

			return true
		})

		if changed {
			out.ChangedFiles = append(out.ChangedFiles, path)

			f, err := os.Create(path)
			if err != nil {
				return err
			}
			defer f.Close()

			err = printer.Fprint(f, fset, node)
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, out, err
	}

	return nil, out, nil
}

func ListImports(ctx context.Context, req *mcp.CallToolRequest, input ListImportsInput) (
	*mcp.CallToolResult,
	ListImportsOutput,
	error,
) {
	mode := packages.NeedSyntax | packages.NeedFiles
	pkgs, err := loadPackagesWithCache(ctx, input.Dir, mode)
	if err != nil {
		return nil, ListImportsOutput{}, err
	}

	out := ListImportsOutput{Imports: []Import{}}

	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			fname := pkg.Fset.File(file.Pos()).Name()
			for _, imp := range file.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				pos := pkg.Fset.Position(imp.Pos())

				out.Imports = append(out.Imports, Import{
					Path: path,
					File: fname,
					Line: pos.Line,
				})
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
	mode := packages.NeedSyntax | packages.NeedFiles
	pkgs, err := loadPackagesWithCache(ctx, input.Dir, mode)
	if err != nil {
		return nil, ListInterfacesOutput{}, err
	}

	out := ListInterfacesOutput{Interfaces: []InterfaceInfo{}}

	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			fname := pkg.Fset.File(file.Pos()).Name()
			ast.Inspect(file, func(n ast.Node) bool {
				ts, ok := n.(*ast.TypeSpec)
				if !ok {
					return true
				}

				if iface, ok := ts.Type.(*ast.InterfaceType); ok {
					pos := pkg.Fset.Position(ts.Pos())
					ifInfo := InterfaceInfo{
						Name:    ts.Name.Name,
						File:    fname,
						Line:    pos.Line,
						Methods: []InterfaceMethod{},
					}

					for _, m := range iface.Methods.List {
						if len(m.Names) > 0 {
							ifInfo.Methods = append(ifInfo.Methods, InterfaceMethod{
								Name: m.Names[0].Name,
								Line: pkg.Fset.Position(m.Pos()).Line,
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

// loadPackagesWithCache loads packages with directory-based caching
func loadPackagesWithCache(ctx context.Context, dir string, mode packages.LoadMode) ([]*packages.Package, error) {
	packageCache.RLock()
	cachedPkgs, exists := packageCache.pkgs[dir]
	packageCache.RUnlock()

	if exists {
		return cachedPkgs, nil
	}

	cfg := &packages.Config{
		Mode:    mode,
		Dir:     dir,
		Context: ctx,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, err
	}

	// Cache the packages
	packageCache.Lock()
	packageCache.pkgs[dir] = pkgs
	packageCache.Unlock()

	return pkgs, nil
}

// getFileLines extracts the lines of a file from the AST for snippet generation
func getFileLines(fset *token.FileSet, file *ast.File) []string {
	filename := fset.File(file.Pos()).Name()
	src, err := os.ReadFile(filename)
	if err != nil {
		return []string{} // Return empty if file can't be read
	}
	return strings.Split(string(src), "\n")
}
