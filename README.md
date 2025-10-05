# Go-Navigator-MCP

Go-Navigator-MCP is a Go-based Model Context Protocol (MCP) server that provides advanced tooling capabilities for Go source code navigation and analysis. It enables AI agents and other tools to perform operations like finding references, listing symbols, renaming identifiers, and exploring Go package structures within a codebase.

## Features

- **List Packages**: Return all Go packages under a given directory
- **List Symbols**: List all functions, structs, interfaces, and interface methods defined in a package
- **Find References**: Find all references (definition and usages) of a given identifier
- **Find Definitions**: Return code locations where a symbol is defined
- **Rename Symbol**: Rename all occurrences of an identifier across Go source files in a directory
- **List Imports**: List all import paths in Go files under a directory
- **List Interfaces**: List all interfaces in Go files under a directory, including their methods

## Optimizations

This project has been optimized for performance and reliability:

- **Consistent API**: Standardized parameter naming and unified parsing methodology across all functions
- **Performance**: Replaced inconsistent parsing methods with `packages.Load` for better performance
- **Context Support**: Added proper context cancellation support for long-running operations
- **Caching**: Implemented package-level caching to avoid redundant parsing operations
- **Memory Efficiency**: Optimized file reading operations to reduce memory usage

## Installation

```bash
go install go-navigator
```

## Usage

### As Standalone Server

Build and run the server:

```bash
# Build the go-navigator executable
go build -o go-navigator ./cmd/go-navigator/main.go

# Run the go-navigator (expects MCP client to connect via stdio)
./go-navigator
```

### As MCP Client

Use with any MCP-compatible client to perform code analysis operations.

### Example Tool Calls

#### List Packages
```json
{
  "name": "listPackages",
  "arguments": {
    "dir": "/path/to/go/project"
  }
}
```

#### List Symbols
```json
{
  "name": "listSymbols",
  "arguments": {
    "dir": "/path/to/go/project",
    "package": "package/path"
  }
}
```

#### Find References
```json
{
  "name": "findReferences",
  "arguments": {
    "dir": "/path/to/go/project",
    "ident": "IdentifierName"
  }
}
```

#### Find Definitions
```json
{
  "name": "findDefinitions",
  "arguments": {
    "dir": "/path/to/go/project",
    "ident": "IdentifierName"
  }
}
```

#### Rename Symbol
```json
{
  "name": "renameSymbol",
  "arguments": {
    "dir": "/path/to/go/project",
    "oldName": "OldIdentifierName",
    "newName": "NewIdentifierName"
  }
}
```

#### List Imports
```json
{
  "name": "listImports",
  "arguments": {
    "dir": "/path/to/go/project"
  }
}
```

#### List Interfaces
```json
{
  "name": "listInterfaces",
  "arguments": {
    "dir": "/path/to/go/project"
  }
}
```

## Architecture

The project is structured as follows:

- `cmd/server/main.go`: Entry point for the MCP server that registers all tools
- `internal/tools/tools.go`: Core implementation of all analysis and refactoring tools
- `internal/tools/tools_test.go`: Comprehensive test suite for all tools
- `internal/tools/testdata/sample/`: Sample Go files used for testing

## Dependencies

The project relies on:
- `github.com/modelcontextprotocol/go-sdk`: Core MCP implementation
- `golang.org/x/tools`: Go analysis tools for package loading and AST manipulation

## Testing

Run all tests:

```bash
# Run all tests
go test ./internal/tools/...

# Run tests with verbose output
go test -v ./internal/tools/...
```

## Protocol Information

This server implements the Model Context Protocol (MCP) and communicates over stdio transport. It is designed to be used as a subprocess that external tools or AI agents can interact with to perform Go code analysis and refactoring tasks.

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Development

When contributing to this project, please follow these guidelines:

1. Maintain consistency with existing code patterns
2. Write tests for new functionality
3. Ensure all tests pass before submitting a pull request
4. Document new features in this README
5. Follow Go best practices and idioms