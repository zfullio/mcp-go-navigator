package main

import (
	"context"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go-navigator/internal/tools"
)

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "go-navigator",
		Version: "v1.0.0",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name: "listPackages",
		Description: `Returns all Go packages under the given directory.
Use this when you need to explore the project structure, discover available packages,
or decide where to search for symbols or references.`,
	}, tools.ListPackages)

	mcp.AddTool(server, &mcp.Tool{
		Name: "listSymbols",
		Description: `Lists all functions, structs, interfaces, and interface methods defined in a package.
Use this when you need to understand what code elements exist inside a package,
prepare to find references, or identify which symbols might be renamed.`,
	}, tools.ListSymbols)

	mcp.AddTool(server, &mcp.Tool{
		Name: "findReferences",
		Description: `Finds all references (definition and usages) of a given identifier across Go source files.
Use this when you need to locate every place where a type, function, or variable is used,
for example before renaming or refactoring.`,
	}, tools.FindReferences)

	mcp.AddTool(server, &mcp.Tool{
		Name: "findDefinitions",
		Description: `Returns the code locations where a symbol is defined (type, function, method, constant, or variable).
Use this when you need to jump to or confirm the exact definition of an identifier.`,
	}, tools.FindDefinitions)

	mcp.AddTool(server, &mcp.Tool{
		Name: "renameSymbol",
		Description: `Performs a precise, scope-aware rename of a Go symbol across the entire project.

Unlike the basic renameSymbol, this version uses go/packages and go/types analysis
to locate all valid references of the target symbol and rename them safely within their scope.

It supports:
  • Dry-run mode (set "dryRun": true) — returns unified diffs instead of modifying files
  • Collision detection — reports name conflicts before applying changes
  • Selective renaming by kind ("type", "func", "var", "const", etc.)

Use this tool when you need reliable, type-safe refactoring rather than text-based search.`,
	}, tools.RenameSymbol)

	mcp.AddTool(server, &mcp.Tool{
		Name: "listImports",
		Description: `Lists all import paths in Go files under a directory.
Use this to review project dependencies, detect unused imports, or identify where external packages are used.`,
	}, tools.ListImports)

	mcp.AddTool(server, &mcp.Tool{
		Name: "listInterfaces",
		Description: `Lists all interfaces in Go files under a directory, including their methods.
Use this to explore contracts in the project, discover points for dependency injection, or identify interfaces that may need mocks for testing.`,
	}, tools.ListInterfaces)

	mcp.AddTool(server, &mcp.Tool{
		Name: "analyzeComplexity",
		Description: `Analyzes Go functions and reports metrics: lines of code, nesting depth, and cyclomatic complexity.
Use this to identify overly complex functions that may need refactoring.`,
	}, tools.AnalyzeComplexity)

	mcp.AddTool(server, &mcp.Tool{
		Name: "deadCode",
		Description: `Finds unused (dead) code in a Go project.
Reports functions, variables, constants, and types that are defined but never used inside the package (ignores exported symbols).`,
	}, tools.DeadCode)

	// Run server
	err := server.Run(context.Background(), &mcp.StdioTransport{})
	if err != nil {
		log.Fatal(err)
	}
}
