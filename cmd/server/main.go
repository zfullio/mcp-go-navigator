package main

import (
	"context"
	"log"

	"go-navigator/internal/tools"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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
		Description: `Renames all occurrences of an identifier across Go source files in a directory.
Use this when you need to perform safe, consistent refactoring by updating a symbol’s name everywhere it appears in the code.`,
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

	// Run server
	err := server.Run(context.Background(), &mcp.StdioTransport{})
	if err != nil {
		log.Fatal(err)
	}
}
