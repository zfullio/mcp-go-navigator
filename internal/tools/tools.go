package tools

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
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
			absPath := pkg.Fset.File(file.Pos()).Name()

			relPath, err := filepath.Rel(input.Dir, absPath)
			if err != nil {
				// fallback: оставить абсолютный путь или обработать по-другому
				relPath = absPath
			}

			ast.Inspect(file, func(n ast.Node) bool {
				if shouldStop(ctx) {
					return false
				}

				switch decl := n.(type) {
				case *ast.FuncDecl:
					out.Symbols = append(out.Symbols, Symbol{
						Kind: "func",
						Name: decl.Name.Name,
						File: relPath,
						Line: pkg.Fset.Position(decl.Pos()).Line,
					})
				case *ast.TypeSpec:
					switch t := decl.Type.(type) {
					case *ast.StructType:
						out.Symbols = append(out.Symbols, Symbol{
							Kind: "struct",
							Name: decl.Name.Name,
							File: relPath,
							Line: pkg.Fset.Position(decl.Pos()).Line,
						})
					case *ast.InterfaceType:
						out.Symbols = append(out.Symbols, Symbol{
							Kind: "interface",
							Name: decl.Name.Name,
							File: relPath,
							Line: pkg.Fset.Position(decl.Pos()).Line,
						})
						// можно дополнительно перечислять методы интерфейса:
						for _, m := range t.Methods.List {
							if len(m.Names) > 0 {
								out.Symbols = append(out.Symbols, Symbol{
									Kind: "method",
									Name: decl.Name.Name + "." + m.Names[0].Name,
									File: relPath,
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
			absPath := pkg.Fset.File(file.Pos()).Name()

			relPath, err := filepath.Rel(input.Dir, absPath)
			if err != nil {
				// fallback: оставить абсолютный путь или обработать по-другому
				relPath = absPath
			}

			lines := getFileLines(pkg.Fset, file)

			ast.Inspect(file, func(n ast.Node) bool {
				if shouldStop(ctx) {
					return false
				}

				if ident, ok := n.(*ast.Ident); ok && ident.Name == input.Ident {
					pos := pkg.Fset.Position(ident.Pos())

					snip := extractSnippet(lines, pos.Line)

					out.References = append(out.References, Reference{
						File:    relPath,
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
			absPath := pkg.Fset.File(file.Pos()).Name()

			relPath, err := filepath.Rel(input.Dir, absPath)
			if err != nil {
				// fallback: оставить абсолютный путь или обработать по-другому
				relPath = absPath
			}

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
							File:    relPath,
							Line:    pos.Line,
							Snippet: snip,
						})
					}
				case *ast.FuncDecl:
					if decl.Name.Name == input.Ident {
						pos := symbolPos(pkg, decl)

						snip := extractSnippet(lines, pos.Line)

						out.Definitions = append(out.Definitions, Definition{
							File:    relPath,
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
								File:    relPath,
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

	// --- Базовая проверка ---
	if input.OldName == input.NewName {
		out.Collisions = append(out.Collisions,
			fmt.Sprintf("cannot rename: %q and %q are the same name", input.OldName, input.NewName))

		return nil, out, nil
	}

	// --- Загружаем пакеты ---
	cfg := &packages.Config{
		Mode:    packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedCompiledGoFiles,
		Dir:     input.Dir,
		Context: ctx,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return fail(out, err)
	}

	if len(pkgs) == 0 {
		return fail(out, fmt.Errorf("no packages loaded from dir: %s", input.Dir))
	}

	// --- Если не dryRun, подгружаем свежие AST (чтобы сбросить изменения после dryRun) ---
	if !input.DryRun {
		pkgs, err = packages.Load(cfg, "./...")
		if err != nil {
			return fail(out, err)
		}
	}

	for _, pkg := range pkgs {
		for i, file := range pkg.Syntax {
			if ctx.Err() != nil {
				return fail(out, ctx.Err())
			}

			filename := pkg.CompiledGoFiles[i]

			origBytes, err := os.ReadFile(filename)
			if err != nil {
				return fail(out, err)
			}

			changed := false

			// --- прямое редактирование AST ---
			ast.Inspect(file, func(n ast.Node) bool {
				if ctx.Err() != nil {
					return false
				}

				// 1️⃣ Переименование типов
				if ts, ok := n.(*ast.TypeSpec); ok {
					if ts.Name.Name == input.OldName && (input.Kind == "" || input.Kind == "type") {
						ts.Name.Name = input.NewName
						changed = true

						return true
					}
				}

				// 2️⃣ Переименование функций и методов
				if fn, ok := n.(*ast.FuncDecl); ok {
					if fn.Name.Name == input.OldName && (input.Kind == "" || input.Kind == "func") {
						fn.Name.Name = input.NewName
						changed = true

						return true
					}
				}

				// 3️⃣ Переименование идентификаторов
				ident, ok := n.(*ast.Ident)
				if !ok || ident.Name != input.OldName {
					return true
				}

				obj := pkg.TypesInfo.ObjectOf(ident)
				if obj == nil {
					return true
				}

				if input.Kind != "" {
					objKind := objStringKind(obj)
					if objKind != input.Kind && objKind != "unknown" {
						return true
					}
				}

				ident.Name = input.NewName
				changed = true

				return true
			})

			if !changed {
				continue
			}

			// --- форматирование через go/format ---
			var buf bytes.Buffer

			ast.SortImports(pkg.Fset, file)

			if err := format.Node(&buf, pkg.Fset, file); err != nil {
				return fail(out, err)
			}

			newContent := buf.Bytes()
			if len(newContent) > 0 && newContent[len(newContent)-1] != '\n' {
				newContent = append(newContent, '\n')
			}

			rel, err := filepath.Rel(input.Dir, filename)
			if err != nil {
				rel = filename
			}

			// всегда фиксируем файл в списке изменений
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

			// --- запись ---
			if err := safeWriteFile(filename, newContent); err != nil {
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
			absPath := pkg.Fset.File(file.Pos()).Name()

			relPath, err := filepath.Rel(input.Dir, absPath)
			if err != nil {
				// fallback: оставить абсолютный путь или обработать по-другому
				relPath = absPath
			}

			for _, imp := range file.Imports {
				if ctxCancelled(ctx) {
					return fail(out, ctx.Err())
				}

				path := strings.Trim(imp.Path.Value, `"`)
				pos := pkg.Fset.Position(imp.Pos())

				out.Imports = append(out.Imports, Import{
					Path: path,
					File: relPath,
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
	mode := packages.NeedSyntax | packages.NeedCompiledGoFiles

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
			absPath := pkg.Fset.File(file.Pos()).Name()

			relPath, err := filepath.Rel(input.Dir, absPath)
			if err != nil {
				// fallback: оставить абсолютный путь или обработать по-другому
				relPath = absPath
			}

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
						File:    relPath,
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

	mode := packages.NeedSyntax | packages.NeedTypes | packages.NeedCompiledGoFiles | packages.NeedTypesInfo

	pkgs, err := loadPackagesWithCache(ctx, input.Dir, mode)
	if err != nil {
		return fail(out, err)
	}

	for _, pkg := range pkgs {
		if shouldStop(ctx) {
			return nil, out, ctx.Err()
		}

		for _, file := range pkg.Syntax {
			absPath := pkg.Fset.File(file.Pos()).Name()

			relPath, err := filepath.Rel(input.Dir, absPath)
			if err != nil {
				// fallback: оставить абсолютный путь или обработать по-другому
				relPath = absPath
			}

			ast.Inspect(file, func(n ast.Node) bool {
				if shouldStop(ctx) {
					return false
				}

				fd, ok := n.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					return true
				}

				pos := pkg.Fset.Position(fd.Pos())
				lines := pkg.Fset.Position(fd.End()).Line - pos.Line

				// запускаем visitor
				visitor := &ComplexityVisitor{
					Ctx:        ctx,
					Fset:       pkg.Fset,
					Nesting:    0,
					MaxNesting: 0,
					Cyclomatic: 1, // минимум = 1
				}
				ast.Walk(visitor, fd.Body)

				out.Functions = append(out.Functions, FunctionComplexity{
					Name:       fd.Name.Name,
					File:       relPath,
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

		// --- Собираем все используемые объекты ---
		used := make(map[types.Object]struct{}, len(pkg.TypesInfo.Uses))
		for _, obj := range pkg.TypesInfo.Uses {
			if obj != nil {
				used[obj] = struct{}{}
			}
		}

		// --- Проверяем определения ---
		for ident, obj := range pkg.TypesInfo.Defs {
			if shouldStop(ctx) {
				return fail(out, ctx.Err())
			}

			if obj == nil {
				continue
			}

			if !isDeadCandidate(ident, obj) {
				continue
			}

			// если объект используется — пропускаем
			if _, ok := used[obj]; ok {
				continue
			}

			pos := pkg.Fset.Position(ident.Pos())
			rel, _ := filepath.Rel(input.Dir, pos.Filename)

			out.Unused = append(out.Unused, DeadSymbol{
				Name: ident.Name,
				Kind: objStringKind(obj),
				File: rel,
				Line: pos.Line,
			})
		}
	}

	return nil, out, nil
}
