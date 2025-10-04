package tools

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"os"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/pmezard/go-difflib/difflib"
	"golang.org/x/tools/go/packages"
)

var bufPool = sync.Pool{New: func() any { return new(bytes.Buffer) }}

func ListPackages(ctx context.Context, req *mcp.CallToolRequest, input ListPackagesInput) (
	*mcp.CallToolResult,
	ListPackagesOutput,
	error,
) {
	cfg := &packages.Config{
		Mode:    packages.NeedName | packages.NeedCompiledGoFiles,
		Dir:     input.Dir,
		Context: ctx,
	}

	out := ListPackagesOutput{Packages: []string{}}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
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
	out := ListSymbolsOutput{Symbols: []Symbol{}}

	mode := packages.NeedSyntax | packages.NeedTypes | packages.NeedCompiledGoFiles | packages.NeedTypesInfo

	pkgs, err := loadPackagesWithCache(ctx, input.Dir, mode)
	if err != nil {
		return fail(out, err)
	}

	for _, pkg := range pkgs {
		if ctxCancelled(ctx) {
			return fail(out, ctx.Err())
		}

		for _, file := range pkg.Syntax {
			fname := pkg.Fset.File(file.Pos()).Name()
			ast.Inspect(file, func(n ast.Node) bool {
				if shouldStop(ctx) {
					return false
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
	out := FindReferencesOutput{References: []Reference{}}

	mode := packages.NeedSyntax | packages.NeedCompiledGoFiles | packages.NeedTypesInfo

	pkgs, err := loadPackagesWithCache(ctx, input.Dir, mode)
	if err != nil {
		return fail(out, err)
	}

	for _, pkg := range pkgs {
		if ctxCancelled(ctx) {
			return fail(out, ctx.Err())
		}

		for _, file := range pkg.Syntax {
			fname := pkg.Fset.File(file.Pos()).Name()
			lines := getFileLines(pkg.Fset, file)

			ast.Inspect(file, func(n ast.Node) bool {
				if shouldStop(ctx) {
					return false
				}

				if ident, ok := n.(*ast.Ident); ok && ident.Name == input.Ident {
					pos := pkg.Fset.Position(ident.Pos())

					snip := extractSnippet(lines, pos.Line)

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
	out := FindDefinitionsOutput{Definitions: []Definition{}}
	mode := packages.NeedSyntax | packages.NeedCompiledGoFiles | packages.NeedTypesInfo

	pkgs, err := loadPackagesWithCache(ctx, input.Dir, mode)
	if err != nil {
		return fail(out, err)
	}

	for _, pkg := range pkgs {
		if ctxCancelled(ctx) {
			return fail(out, ctx.Err())
		}

		for _, file := range pkg.Syntax {
			fname := pkg.Fset.File(file.Pos()).Name()
			lines := getFileLines(pkg.Fset, file)

			ast.Inspect(file, func(n ast.Node) bool {
				if shouldStop(ctx) {
					return false
				}

				switch decl := n.(type) {
				case *ast.TypeSpec:
					if decl.Name.Name == input.Ident {
						pos := symbolPos(pkg, decl)

						snip := extractSnippet(lines, pos.Line)

						out.Definitions = append(out.Definitions, Definition{
							File:    fname,
							Line:    pos.Line,
							Snippet: snip,
						})
					}
				case *ast.FuncDecl:
					if decl.Name.Name == input.Ident {
						pos := symbolPos(pkg, decl)

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
							pos := symbolPos(pkg, decl)

							snip := extractSnippet(lines, pos.Line)

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
	out := RenameSymbolOutput{
		ChangedFiles: []string{},
		Diffs:        []FileDiff{},
		Collisions:   []string{},
	}

	if input.OldName == input.NewName {
		out.Collisions = append(out.Collisions,
			fmt.Sprintf("cannot rename: %q and %q are the same name", input.OldName, input.NewName))

		return nil, out, nil
	}

	pkgs, err := loadPackagesWithCache(ctx, input.Dir,
		packages.NeedSyntax|packages.NeedTypes|packages.NeedTypesInfo|packages.NeedCompiledGoFiles)
	if err != nil {
		return fail(out, err)
	}

	if len(pkgs) == 0 {
		return fail(out, fmt.Errorf("no packages loaded from dir: %s", input.Dir))
	}

	for _, pkg := range pkgs {
		if ctx.Err() != nil {
			return fail(out, ctx.Err())
		}

		for i, file := range pkg.Syntax {
			fset := pkg.Fset
			filename := pkg.CompiledGoFiles[i]

			// ✅ создаём независимую копию AST, чтобы не мутировать кэш
			cloned := astCopy(file)

			origBytes, err := os.ReadFile(filename)
			if err != nil {
				return fail(out, err)
			}

			changed := false

			ast.Inspect(cloned, func(n ast.Node) bool {
				if ctx.Err() != nil {
					return false
				}

				// --- Fallback: TypeSpec для объявлений типов ---
				if ts, ok := n.(*ast.TypeSpec); ok {
					if ts.Name.Name == input.OldName && (input.Kind == "" || input.Kind == "type") {
						// Проверка коллизии
						if ts.Name.Name == input.NewName {
							pos := fset.Position(ts.Pos())
							out.Collisions = append(out.Collisions,
								fmt.Sprintf("%s:%d (type name already %s)", pos.Filename, pos.Line, input.NewName))

							return true
						}

						ts.Name.Name = input.NewName
						changed = true

						return true
					}
				}

				// --- Основной вариант: любые идентификаторы ---
				ident, ok := n.(*ast.Ident)
				if !ok || ident.Name != input.OldName {
					return true
				}

				obj := pkg.TypesInfo.ObjectOf(ident)
				if obj == nil {
					return true
				}

				// Фильтр по виду (если задан)
				if input.Kind != "" && objStringKind(obj) != input.Kind {
					return true
				}

				// Проверка коллизий в области видимости
				if obj.Parent() != nil {
					if other := obj.Parent().Lookup(input.NewName); other != nil {
						pos := fset.Position(ident.Pos())
						out.Collisions = append(out.Collisions,
							fmt.Sprintf("%s:%d (conflict with %s)", pos.Filename, pos.Line, other.Name()))

						return true
					}
				}

				ident.Name = input.NewName
				changed = true

				return true
			})

			if changed {
				var buf bytes.Buffer
				err := printer.Fprint(&buf, fset, cloned)
				if err != nil {
					return fail(out, err)
				}

				newContent := buf.Bytes()

				if input.DryRun {
					diff := difflib.UnifiedDiff{
						A:        difflib.SplitLines(string(origBytes)),
						B:        difflib.SplitLines(string(newContent)),
						FromFile: "before",
						ToFile:   "after",
						Context:  3,
					}
					text, _ := difflib.GetUnifiedDiffString(diff)
					out.Diffs = append(out.Diffs, FileDiff{Path: filename, Diff: text})
				} else {
					err := safeWriteFile(filename, newContent)
					if err != nil {
						return fail(out, err)
					}

					out.ChangedFiles = append(out.ChangedFiles, filename)
				}
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
	mode := packages.NeedSyntax | packages.NeedCompiledGoFiles

	pkgs, err := loadPackagesWithCache(ctx, input.Dir, mode)
	if err != nil {
		return nil, ListImportsOutput{}, err
	}

	out := ListImportsOutput{Imports: []Import{}}

	for _, pkg := range pkgs {
		if ctxCancelled(ctx) {
			return fail(out, ctx.Err())
		}

		for _, file := range pkg.Syntax {
			fname := pkg.Fset.File(file.Pos()).Name()
			for _, imp := range file.Imports {
				if ctxCancelled(ctx) {
					return fail(out, ctx.Err())
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
				if shouldStop(ctx) {
					return false
				}

				ts, ok := n.(*ast.TypeSpec)
				if !ok {
					return true
				}

				if iface, ok := ts.Type.(*ast.InterfaceType); ok {
					pos := symbolPos(pkg, ts)
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
		return fail(out, err)
	}

	for _, pkg := range pkgs {
		if shouldStop(ctx) {
			return nil, out, ctx.Err()
		}

		for fname, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				if shouldStop(ctx) {
					return false
				}

				fd, ok := n.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					return true
				}

				pos := fset.Position(fd.Pos())
				lines := fset.Position(fd.End()).Line - pos.Line

				// запускаем visitor
				visitor := &ComplexityVisitor{
					Ctx:        ctx,
					Fset:       fset,
					Nesting:    0,
					MaxNesting: 0,
					Cyclomatic: 1, // минимум = 1
				}
				ast.Walk(visitor, fd.Body)

				out.Functions = append(out.Functions, FunctionComplexity{
					Name:       fd.Name.Name,
					File:       fname,
					Line:       pos.Line,
					Lines:      lines,
					Nesting:    visitor.MaxNesting,
					Cyclomatic: visitor.Cyclomatic,
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
	out := DeadCodeOutput{Unused: []DeadSymbol{}}

	cfg := &packages.Config{
		Mode:    packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedCompiledGoFiles,
		Dir:     input.Dir,
		Context: ctx,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return fail(out, err)
	}

	for _, pkg := range pkgs {
		if shouldStop(ctx) {
			return fail(out, ctx.Err())
		}

		// собираем все использования
		used := make(map[types.Object]struct{}, len(pkg.TypesInfo.Uses))
		for _, obj := range pkg.TypesInfo.Uses {
			if obj != nil {
				used[obj] = struct{}{}
			}
		}

		// проверяем определения
		for ident, obj := range pkg.TypesInfo.Defs {
			if shouldStop(ctx) {
				return fail(out, ctx.Err())
			}

			if obj == nil {
				continue
			}
			// пропускаем экспортируемые символы
			if ast.IsExported(ident.Name) {
				continue
			}

			if _, ok := used[obj]; !ok {
				pos := pkg.Fset.Position(ident.Pos())
				out.Unused = append(out.Unused, DeadSymbol{
					Name: ident.Name,
					Kind: objStringKind(obj),
					File: pos.Filename,
					Line: pos.Line,
				})
			}
		}
	}

	return nil, out, nil
}
