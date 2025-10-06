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
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/pmezard/go-difflib/difflib"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
)

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
