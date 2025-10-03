package tools

import (
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/tools/go/packages"
)

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
		if ctxCancelled(ctx) {
			return nil, out, ctx.Err()
		}

		for _, file := range pkg.Syntax {
			fname := pkg.Fset.File(file.Pos()).Name()
			ast.Inspect(file, func(n ast.Node) bool {
				// Check context cancellation during AST traversal
				select {
				case <-ctx.Done():
					return false
				default:
				}

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
		if ctxCancelled(ctx) {
			return nil, out, ctx.Err()
		}

		for _, file := range pkg.Syntax {
			fname := pkg.Fset.File(file.Pos()).Name()
			lines := getFileLines(pkg.Fset, file)

			ast.Inspect(file, func(n ast.Node) bool {
				// Check context cancellation during AST traversal
				select {
				case <-ctx.Done():
					return false
				default:
				}

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
		if ctxCancelled(ctx) {
			return nil, out, ctx.Err()
		}

		for _, file := range pkg.Syntax {
			fname := pkg.Fset.File(file.Pos()).Name()
			lines := getFileLines(pkg.Fset, file)

			ast.Inspect(file, func(n ast.Node) bool {
				// Check context cancellation during AST traversal
				select {
				case <-ctx.Done():
					return false
				default:
				}

				switch decl := n.(type) {
				case *ast.TypeSpec:
					if decl.Name.Name == input.Ident {
						pos := pkg.Fset.Position(decl.Pos())

						snip := extractSnippet(lines, pos.Line)

						out.Definitions = append(out.Definitions, Definition{
							File:    fname,
							Line:    pos.Line,
							Snippet: snip,
						})
					}
				case *ast.FuncDecl:
					if decl.Name.Name == input.Ident {
						pos := pkg.Fset.Position(decl.Pos())

						snip := extractSnippet(lines, pos.Line)

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

		// Read original content to compare later
		originalContent, err := os.ReadFile(path)
		if err != nil {
			return err
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
			// Write the modified AST to buffer to compare with original
			var buf bytes.Buffer
			err = printer.Fprint(&buf, fset, node)
			if err != nil {
				return err
			}

			newContent := buf.Bytes()

			// Only write if content actually changed
			if !bytes.Equal(originalContent, newContent) {
				out.ChangedFiles = append(out.ChangedFiles, path)

				f, err := os.Create(path)
				if err != nil {
					return err
				}
				defer f.Close()

				_, err = f.Write(newContent)
				if err != nil {
					return err
				}
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
		if ctxCancelled(ctx) {
			return nil, out, ctx.Err()
		}

		for _, file := range pkg.Syntax {
			fname := pkg.Fset.File(file.Pos()).Name()
			for _, imp := range file.Imports {
				// Check context cancellation during processing
				select {
				case <-ctx.Done():
					return nil, out, ctx.Err()
				default:
				}

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
		if ctxCancelled(ctx) {
			return nil, out, ctx.Err()
		}

		for _, file := range pkg.Syntax {
			fname := pkg.Fset.File(file.Pos()).Name()
			ast.Inspect(file, func(n ast.Node) bool {
				// Check context cancellation during AST traversal
				select {
				case <-ctx.Done():
					return false
				default:
				}

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

func AnalyzeComplexity(ctx context.Context, req *mcp.CallToolRequest, input AnalyzeComplexityInput) (
	*mcp.CallToolResult,
	AnalyzeComplexityOutput,
	error,
) {
	out := AnalyzeComplexityOutput{Functions: []FunctionComplexity{}}

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, input.Dir, nil, parser.ParseComments)
	if err != nil {
		return nil, out, err
	}

	for _, pkg := range pkgs {
		// Check context cancellation between packages
		select {
		case <-ctx.Done():
			return nil, out, ctx.Err()
		default:
		}

		for fname, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				// Check context cancellation during AST traversal
				select {
				case <-ctx.Done():
					return false
				default:
				}

				fd, ok := n.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					return true
				}

				pos := fset.Position(fd.Pos())
				lines := fset.Position(fd.End()).Line - pos.Line

				// метрики
				nesting := 0
				maxNesting := 0
				cyclomatic := 1 // минимум = 1

				ast.Inspect(fd.Body, func(n ast.Node) bool {
					// Check context cancellation during inner AST traversal
					select {
					case <-ctx.Done():
						return false
					default:
					}

					switch n.(type) {
					case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
						nesting++
						if nesting > maxNesting {
							maxNesting = nesting
						}
						cyclomatic++
					case *ast.CaseClause:
						cyclomatic++
					}
					return true
				})

				out.Functions = append(out.Functions, FunctionComplexity{
					Name:       fd.Name.Name,
					File:       fname,
					Line:       pos.Line,
					Lines:      lines,
					Nesting:    maxNesting,
					Cyclomatic: cyclomatic,
				})

				return true
			})
		}
	}

	return nil, out, nil
}

func DeadCode(ctx context.Context, req *mcp.CallToolRequest, input DeadCodeInput) (
	*mcp.CallToolResult,
	DeadCodeOutput,
	error,
) {
	out := DeadCodeOutput{Unused: []DeadSymbol{}}

	cfg := &packages.Config{
		Mode:    packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedFiles,
		Dir:     input.Dir,
		Context: ctx,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, out, err
	}

	for _, pkg := range pkgs {
		if ctxCancelled(ctx) {
			return nil, out, ctx.Err()
		}

		used := map[types.Object]bool{}

		// Все использования
		for _, obj := range pkg.TypesInfo.Uses {
			if obj != nil {
				used[obj] = true
			}
		}

		// Проверяем определения
		for ident, obj := range pkg.TypesInfo.Defs {
			// Check context cancellation during processing
			select {
			case <-ctx.Done():
				return nil, out, ctx.Err()
			default:
			}

			if obj == nil {
				continue
			}

			// Пропускаем экспортируемые символы (начинаются с заглавной буквы)
			if ast.IsExported(ident.Name) {
				continue
			}

			if _, ok := used[obj]; !ok {
				pos := pkg.Fset.Position(ident.Pos())
				kind := "var"
				switch obj.(type) {
				case *types.Func:
					kind = "func"
				case *types.TypeName:
					kind = "type"
				case *types.Const:
					kind = "const"
				}

				out.Unused = append(out.Unused, DeadSymbol{
					Name: ident.Name,
					Kind: kind,
					File: pos.Filename,
					Line: pos.Line,
				})
			}
		}
	}

	return nil, out, nil
}
