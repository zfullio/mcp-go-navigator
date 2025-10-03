# Go-Help-MCP Project Guide

## Project Overview

The `go-help-mcp` project is a Go-based implementation of an MCP (Model Context Protocol) server that provides advanced tooling capabilities for Go source code navigation and analysis. It enables AI agents and other tools to perform operations like finding references, listing symbols, renaming identifiers, and exploring Go package structures within a codebase.

The project implements the Model Context Protocol (MCP) specification and exposes a set of powerful Go analysis tools that facilitate code navigation, refactoring, and exploration. The server is designed to be used by AI assistants or other automated tools that need to understand and manipulate Go code.

## Architecture

The project is structured as follows:

- `cmd/server/main.go`: Entry point for the MCP server that registers all tools
- `internal/tools/tools.go`: Core implementation of all analysis and refactoring tools
- `internal/tools/tools_test.go`: Comprehensive test suite for all tools
- `internal/tools/testdata/sample/`: Sample Go files used for testing
- `go.mod`/`go.sum`: Go module definitions and dependencies
- `.idea/`: IDE configuration files (IntelliJ/GoLand)

## Complete Directory Structure

```
/home/viktor-d/Programming/MyProjects/go-help-mcp/
├───.idea/                    # Конфигурационные файлы IDE
│   ├───.gitignore
│   ├───go-help-mcp.iml
│   ├───golinter.xml
│   ├───inspectionProfiles/
│   │   └───Project_Default.xml
│   ├───material_theme_project_new.xml
│   ├───modules.xml
│   ├───vcs.xml
│   └───workspace.xml
├───cmd/                      # Точка входа приложения
│   └───server/
│       └───main.go           # Основной файл сервера MCP
├───internal/                 # Внутренние пакеты (не для импорта извне)
│   └───tools/                # Реализация инструментов анализа Go кода
│       ├───tools.go          # Основная реализация всех инструментов
│       ├───tools_test.go     # Тесты для инструментов
│       └───testdata/         # Тестовые данные
│           └───sample/       # Примеры Go файлов для тестирования
│               ├───bar.go
│               └───foo.go
├───AGENTS.md                 # Информационный файл о проекте (новый)
├───go-navigator              # Исполняемый файл (скомпилированный)
├───go.mod                    # Модуль Go
├───go.sum                    # Контрольные суммы зависимостей
└───service                   # Исполняемый файл (скомпилированный)
```

## Tools Provided

The server exposes the following tools:

### `listPackages`
- **Purpose**: Returns all Go packages under a given directory
- **Use case**: Explore project structure, discover available packages
- **Input**: `dir` (directory to scan for packages)

### `listSymbols`
- **Purpose**: Lists all functions, structs, interfaces, and interface methods defined in a package
- **Use case**: Understand code elements within a package, prepare for refactoring
- **Input**: `dir` (directory to scan), `package` (package path to inspect)

### `findReferences`
- **Purpose**: Finds all references (definition and usages) of a given identifier
- **Use case**: Locate every place where a type, function, or variable is used
- **Input**: `dir` (directory to scan), `ident` (identifier to search for)

### `findDefinitions`
- **Purpose**: Returns code locations where a symbol is defined
- **Use case**: Jump to or confirm the exact definition of an identifier
- **Input**: `dir` (directory to scan), `ident` (identifier to search for definition)

### `renameSymbol`
- **Purpose**: Renames all occurrences of an identifier across Go source files in a directory
- **Use case**: Perform safe, consistent refactoring across codebase
- **Input**: `dir` (directory to scan), `oldName` (symbol name to rename), `newName` (new symbol name)

## Building and Running

### Prerequisites
- Go 1.25 or higher

### Build
```bash
# Build the server executable
go build -o go-navigator ./cmd/server/main.go
```

### Run
```bash
# Run the server (expects MCP client to connect via stdio)
./go-navigator
```

### Testing
```bash
# Run all tests
go test ./internal/tools/...

# Run tests with verbose output
go test -v ./internal/tools/...
```

## Dependencies

The project relies on:
- `github.com/modelcontextprotocol/go-sdk`: Core MCP implementation
- `golang.org/x/tools`: Go analysis tools for package loading and AST manipulation

## Development Conventions

- All tools follow a consistent input/output structure with JSON serialization
- Error handling is implemented according to Go best practices
- Tests use sample code to validate tool functionality
- Tools support context for cancellation and timeouts
- All file operations respect the provided directory scope

## Use Cases

This project is designed to be used by AI coding assistants or IDE extensions that need to:
- Understand Go code structure
- Perform refactoring operations
- Navigate between definitions and references
- Explore package organization
- Analyze code elements in a programmatic way

## Testing

The project includes comprehensive tests that validate each tool against sample Go code in `internal/tools/testdata/sample/`. Tests verify that:
- Packages are correctly listed
- Symbols are properly identified and categorized
- References and definitions are accurately located
- Symbol renaming works without breaking code

## Protocol Information

This server implements the Model Context Protocol (MCP) and communicates over stdio transport. It is designed to be used as a subprocess that external tools or AI agents can interact with to perform Go code analysis and refactoring tasks.

## Usage

The go-navigator executable provides a powerful set of tools for Go code analysis that can be used programmatically or through an MCP client:

### Direct Tool Usage
Inside environments that support it (like this one), you can use the following tools directly:

- **`listPackages`**: Discover all Go packages in a directory
- **`listSymbols`**: Get all functions, structs, interfaces, and methods in a package  
- **`findReferences`**: Locate all usages of a specific identifier
- **`findDefinitions`**: Find where symbols are defined
- **`renameSymbol`**: Safely rename identifiers across your codebase

### When to Use go-navigator
- When exploring unfamiliar Go codebases
- When you need to understand relationships between code elements
- When performing refactoring operations that require knowledge of all symbol usages
- When building tools that need to analyze Go source code
- When you want accurate Go AST-based analysis instead of regex/grep approaches

### Tool Best Practices
When using these tools, always:
1. Use the full directory path where your Go code resides
2. For package-specific tools, provide the full package import path
3. Use `listPackages` first to discover available packages in your codebase
4. Use `listSymbols` to understand what's in a specific package before searching for references
5. Remember that all tools operate within the specified directory scope for security

Example usage flow:
1. First list packages: `listPackages(dir="/path/to/code")`
2. Then list symbols in a specific package: `listSymbols(dir="/path/to/code", package="package/path")`
3. Find references to specific identifiers: `findReferences(dir="/path/to/code", ident="IdentifierName")`
4. Rename symbols safely: `renameSymbol(dir="/path/to/code", oldName="OldName", newName="NewName")`

This toolset provides a robust foundation for Go code analysis and manipulation, leveraging Go's own AST parsing capabilities for accuracy.

## Code Quality Notes

After reviewing the implementation, I found that the code is working correctly and all tests pass. The project builds successfully, all tests pass with `go test`, and `go vet` doesn't detect any issues. However, there are some areas that could be improved:

1. **Consistency**: The FindDefinitionsInput and FindReferencesInput use different parameter names (`IdentName` vs `ident` in the JSON tags), which could be made more consistent.

2. **Error Handling**: The RenameSymbol function has good error handling, but error messages could be more descriptive in some cases.

3. **Functionality**: The ListSymbols function properly handles structs, interfaces, functions and methods, which covers the main Go language constructs.

4. **Performance**: For large codebases, FindReferences and FindDefinitions could potentially be optimized by using the packages.Load API consistently instead of mixing it with parser.ParseDir.

5. **Documentation**: The code is well-documented with JSON schema tags which helps with MCP integration.


## Go Project Error Analysis (with go-navigator)

When analyzing a Go project for errors or suspicious code, always use the **go-navigator MCP server** instead of parsing files manually. Follow this workflow:

---

### 🔎 Step-by-step workflow

1. **Explore project structure**
    - Tool: `listPackages`
    - Goal: discover all Go packages under the root directory.
    - Use this to build a map of the project before diving deeper.

2. **Inspect package contents**
    - Tool: `listSymbols` (for each package)
    - Goal: list functions, structs, and interfaces.
    - Check for:
        - packages with too many symbols (possible "god packages"),
        - exported symbols (capitalized) with no documentation,
        - overly large or generic functions.

3. **Analyze dependencies**
    - Tool: `listImports` (for each package)
    - Goal: list import paths used.
    - Check for:
        - unused or duplicate imports,
        - suspicious third-party dependencies,
        - inconsistent logging or formatting libraries.

4. **Review abstractions**
    - Tool: `listInterfaces`
    - Goal: find all interfaces and their methods.
    - Check for:
        - interfaces with no implementations,
        - interfaces with a single trivial method (replace with function?),
        - very large interfaces (violate SRP).

5. **Drill down on suspicious symbols**
    - Tool: `findDefinitions`  
      → locate the source definition of a symbol.
    - Tool: `findReferences`  
      → list all usage locations of that symbol.
    - Use these when you need to confirm how a symbol is defined or if it's over-used across the codebase.

6. **Check refactoring feasibility**
    - Tool: `renameSymbol` (optional, preview changes)
    - Goal: test how safely a symbol can be renamed.
    - Use this for generic names like `Do`, `Handle`, `Manager` to suggest better alternatives.

---

### ✅ Agent guidelines

- **Always prefer go-navigator tools** for project analysis.
- **Start broad** (`listPackages`, `listSymbols`) before doing focused checks (`findDefinitions`, `findReferences`).
- **Combine imports + interfaces** analysis to detect architectural issues.
- **Propose renames or extractions** only after verifying usage with `findReferences`.



## Qwen Added Memories
- When working with Go code in this project, I should use the go-navigator tools (listPackages, listSymbols, findReferences, findDefinitions, renameSymbol) to analyze Go code instead of parsing source files directly with other tools like grep, rg, ast-grep, etc. The go-navigator provides accurate Go AST-based analysis.
