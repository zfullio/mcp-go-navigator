# Go-Navigator-MCP

Go-Navigator-MCP is a Go-based Model Context Protocol (MCP) server that provides advanced tooling capabilities for Go source code navigation and analysis. It enables AI agents and other tools to perform operations like finding references, listing symbols, renaming identifiers, and exploring Go package structures within a codebase.

## Features

- **List Packages**: Return all Go packages under a given directory
- **List Symbols**: List all functions, structs, interfaces, and interface methods defined in a package
- **Find References**: Find all references (definition and usages) of a given identifier, grouped by file with pagination support
- **Find Definitions**: Return code locations where a symbol is defined, grouped by file with pagination support
- **Find Best Context**: Return a focused context bundle for a symbol: primary definition, key usages, test coverage, and its direct imports
- **Find Implementations**: Show which concrete types implement interfaces (and vice versa)
- **Rename Symbol**: Rename all occurrences of an identifier across Go source files in a directory
- **List Imports**: List all import paths in Go files under a directory
- **List Interfaces**: List all interfaces in Go files under a directory, including their methods
- **Project Schema**: Aggregate full structural metadata of a Go module with configurable detail levels (summary, standard, deep)
- **Analyze Complexity**: Analyze function metrics including cyclomatic complexity and nesting depth
- **Detect Dead Code**: Find unused functions, variables, constants, and types within the Go project
- **Analyze Dependencies**: Build a graph of dependencies between internal packages with fan-in/fan-out and cycle detection
- **Metrics Summary**: Aggregate project metrics including package/struct/interface counts, average complexity, and unused code ratios
- **AST Rewrite**: Pattern-driven AST transformations with type-aware understanding
- **Read Function Source**: Get full source code and metadata of a Go function or method by name
- **Read File**: Get package metadata, imports, and declared symbols from a Go source file
- **Read Struct**: Get struct declaration including fields, tags, comments, and optionally associated methods

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
The `package` argument should match the module-qualified path reported by `go list`.
```json
{
  "name": "listSymbols",
  "arguments": {
    "dir": "/path/to/go/project",
    "package": "your-module/internal/tools"
  }
}
```

#### Find References
```json
{
  "name": "findReferences",
  "arguments": {
    "dir": "/path/to/go/project",
    "ident": "IdentifierName",
    "limit": 25,
    "offset": 0
  }
}
```
Results include a `total` count and are grouped by file to reduce duplication. Omit `limit`/`offset` (or set `limit` to 0) to stream the full set.

#### Find Definitions
```json
{
  "name": "findDefinitions",
  "arguments": {
    "dir": "/path/to/go/project",
    "ident": "IdentifierName",
    "limit": 25,
    "offset": 0
  }
}
```
Output mirrors `findReferences`: per-file groupings with a `total` count and pagination controls.

#### Find Best Context
```json
{
  "name": "findBestContext",
  "arguments": {
    "dir": "/path/to/go/project",
    "ident": "IdentifierName",
    "kind": "func",
    "maxUsages": 3,
    "maxTestUsages": 2,
    "maxDependencies": 5
  }
}
```
Returns a focused context bundle with the symbol's definition, key usages, and direct imports.

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
Optionally restrict results by package path (use the value from `go list`).
```json
{
  "name": "listImports",
  "arguments": {
    "dir": "/path/to/go/project",
    "package": "your-module/internal/tools"
  }
}
```

#### List Interfaces
Optionally restrict results by package path (use the value from `go list`).
```json
{
  "name": "listInterfaces",
  "arguments": {
    "dir": "/path/to/go/project",
    "package": "your-module/internal/tools"
  }
}
```

#### Project Schema
```json
{
  "name": "projectSchema",
  "arguments": {
    "dir": "/path/to/go/project",
    "depth": "standard"
  }
}
```

#### Analyze Complexity
```json
{
  "name": "analyzeComplexity",
  "arguments": {
    "dir": "/path/to/go/project",
    "package": "module/internal/tools"
  }
}
```

#### Detect Dead Code
```json
{
  "name": "deadCode",
  "arguments": {
    "dir": "/path/to/go/project",
    "package": "module/internal/tools",
    "includeExported": true,
    "limit": 10
  }
}
```

#### Analyze Dependencies
```json
{
  "name": "analyzeDependencies",
  "arguments": {
    "dir": "/path/to/go/project",
    "package": "module/internal/tools"
  }
}
```

#### Find Implementations
```json
{
  "name": "findImplementations",
  "arguments": {
    "dir": "/path/to/go/project",
    "name": "InterfaceName"
  }
}
```

#### Metrics Summary
```json
{
  "name": "metricsSummary",
  "arguments": {
    "dir": "/path/to/go/project",
    "package": "module/internal/tools"
  }
}
```

#### AST Rewrite
```json
{
  "name": "astRewrite",
  "arguments": {
    "dir": "/path/to/go/project",
    "find": "oldPattern(x)",
    "replace": "newPattern(x)",
    "dryRun": true
  }
}
```

#### Read Function Source
```json
{
  "name": "readFunc",
  "arguments": {
    "dir": "/path/to/go/project",
    "name": "FunctionName"
  }
}
```

#### Read Go File
```json
{
  "name": "readGoFile",
  "arguments": {
    "dir": "/path/to/go/project",
    "file": "relative/path/to/file.go",
    "mode": "summary"
  }
}
```

#### Read Struct
```json
{
  "name": "readStruct",
  "arguments": {
    "dir": "/path/to/go/project",
    "name": "StructName",
    "includeMethods": true
  }
}
```

## Architecture

The project is structured as follows:

- `cmd/go-navigator/main.go`: Entry point for the MCP server that wires every MCP tool
- `internal/tools/listers.go`: Listing helpers (`listPackages`, `listSymbols`, `listImports`, `listInterfaces`)
- `internal/tools/finders.go`: Definition/reference discovery (`findDefinitions`, `findReferences`, `findBestContext`, `findImplementations`)
- `internal/tools/analyzers.go`: Metrics and diagnostics (`metricsSummary`, `analyzeComplexity`, `deadCode`, `analyzeDependencies`)
- `internal/tools/refactorers.go`: Write-capable flows such as `renameSymbol` and `astRewrite`
- `internal/tools/readers.go`: Source extraction helpers (`readGoFile`, `readFunc`, `readStruct`)
- `internal/tools/cache.go`, `helpers.go`, `logging.go`, `descriptions.go`: Shared infrastructure, logging, and tool metadata
- `internal/tools/*_test.go` (e.g., `listers_test.go`, `finders_test.go`, `refactorers_test.go`): Decomposed test suites for each tool category.
- `internal/tools/testdata/sample/`: Sample Go files used for testing

## Dependencies

The project relies on:
- `github.com/modelcontextprotocol/go-sdk`: Core MCP implementation
- `golang.org/x/tools`: Go analysis tools for package loading and AST manipulation

## Testing

The test suite is decomposed for targeted testing of individual tools. Run all tests:

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
