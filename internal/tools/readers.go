package tools

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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
	start := logStart("ReadFunc", logFields(
		input.Dir,
		newLogField("name", input.Name),
	))
	out := ReadFuncOutput{}

	defer func() { logEnd("ReadFunc", start, 1) }()

	mode := loadModeSyntaxTypesNamed

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
		for _, astFile := range pkg.Syntax {
			fset := pkg.Fset

			ast.Inspect(astFile, func(n ast.Node) bool {
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

				// Вычисляем позицию найденной функции
				startPos := fset.Position(fd.Pos())
				endPos := fset.Position(fd.End())

				// Определяем абсолютный путь к файлу, в котором находится функция
				// Сначала пытаемся получить имя файла из FileSet
				abs := ""

				funcFile := fset.File(fd.Pos())
				if funcFile != nil {
					abs = funcFile.Name()
				}

				// Если имя файла не получено из FileSet, используем резервный способ
				if abs == "" {
					// Сопоставляем позицию функции с файлами в пакете
					funcPos := fd.Pos()
					for _, compiledGoFile := range pkg.CompiledGoFiles {
						// Пытаемся открыть и разобрать каждый файл, чтобы проверить,
						// содержится ли в нем функция с заданной позицией
						if fset.File(funcPos).Name() == compiledGoFile {
							abs = compiledGoFile

							break
						}
					}
				}

				// Если после всех попыток abs все еще пустой, используем первый файл из пакета как запасной вариант
				if abs == "" && len(pkg.CompiledGoFiles) > 0 {
					abs = pkg.CompiledGoFiles[0]
				}

				rel := relativePath(input.Dir, abs)
				if rel == "" && abs != "" {
					rel = filepath.ToSlash(abs)
				}

				// Ensure rel is not empty - this is a fallback to prevent empty File field
				if rel == "" {
					// В крайнем случае, если все методы дали пустой результат, используем первый файл из пакета
					if len(pkg.CompiledGoFiles) > 0 {
						rel = pkg.CompiledGoFiles[0]
					} else {
						rel = abs
					}
				}

				var buf bytes.Buffer

				err = format.Node(&buf, fset, fd)
				if err != nil {
					logError("ReadFunc", err, "failed to format function")

					return false
				}

				// Используем имя пакета из текущего файла как резервный вариант, если pkg.PkgPath пустой
				packageName := pkg.PkgPath
				if packageName == "" {
					packageName = astFile.Name.Name
				}

				out.Function = FunctionSource{
					Name:       fd.Name.Name,
					Receiver:   recv,
					Package:    packageName,
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
	start := logStart("ReadFile", logFields(
		input.Dir,
		newLogField("file", input.File),
		newLogField("mode", input.Mode),
	))
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
	out.Symbols = append(out.Symbols, collectSymbols(file, fset, out.Package, input.File)...)

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
	start := logStart("ReadStruct", logFields(
		input.Dir,
		newLogField("name", input.Name),
	))
	out := ReadStructOutput{}

	defer func() { logEnd("ReadStruct", start, 1) }()

	mode := loadModeSyntaxTypesNamedFiles

	pkgs, _, err := loadFilteredPackages(ctx, input.Dir, mode, "", "ReadStruct")
	if err != nil {
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

		for i, file := range pkg.Syntax {
			fset := pkg.Fset
			relPath := resolveFilePath(pkg, input.Dir, i, file)

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
