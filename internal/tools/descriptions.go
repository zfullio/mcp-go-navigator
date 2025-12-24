package tools

// Centralized tool descriptions for Go Navigator MCP server.
// Each description is concise, structured, and suitable for AI and human use.

// ListPackagesDesc describes the listPackages tool.
const ListPackagesDesc = `
Returns all Go packages under the given directory.

Use when:
- You need to understand the module structure or find where code lives.
- Typically the first step before analyzing symbols or dependencies.

Example:
listPackages { "dir": "." }
`

// ListSymbolsDesc describes the listSymbols tool.
const ListSymbolsDesc = `
Lists all functions, structs, interfaces, and methods in a Go package.

Use when:
- You want to explore available code entities before refactoring or documentation.
- Useful before using getDefinitions or renameSymbol.

Notes:
- Provide the package path exactly as reported by 'go list' (for example, 'go-navigator/internal/tools').

Example:
listSymbols { "dir": ".", "package": "go-navigator/internal/tools" }
`

// GetDefinitionsDesc describes the getDefinitions tool.
const GetDefinitionsDesc = `
Locates where a symbol is defined (type, func, var, const).

Use when:
- You need to find where a symbol is declared or defined.
- Ideal for navigation, documentation, or analysis.

Notes:
- Results are grouped by file with total counts.
- Use optional "limit" and "offset" to paginate large result sets.

Example:
getDefinitions { "dir": ".", "ident": "TaskService" }
`

// GetReferencesDesc describes the getReferences tool.
const GetReferencesDesc = `
Finds all references and usages of an identifier using go/types semantic analysis.

Use when:
- You want to check where a symbol is used before renaming or deleting it.
- Ideal companion to getDefinitions.

Notes:
- Results are grouped by file with total counts.
- Use optional "limit" and "offset" to paginate large result sets.

Example:
getReferences { "dir": ".", "ident": "TaskService" }
`

// GetSymbolContextDesc describes the getSymbolContext tool.
const GetSymbolContextDesc = `
Returns a focused context bundle for a symbol: primary definition, key usages, test coverage, and its direct imports.

Use when:
- You want a quick, token-efficient snapshot before diving deeper into the code.
- You need to reason about a symbol and avoid scanning entire files manually.

Notes:
- Usages are prioritised to surface the top non-test (default 3) and test (default 2) references; limits are configurable.
- Dependencies list the imports taken from the definition files, trimmed to the most relevant entries.

Example:
getSymbolContext { "dir": ".", "ident": "DoSomething", "kind": "func" }
`

// RenameSymbolDesc describes the renameSymbol tool.
const RenameSymbolDesc = `
Performs a safe, scope-aware rename with dry-run diff and collision detection.

Use when:
- You need to rename a function, struct, or variable across a module safely.
- Always test with "dryRun": true first.

Example:
renameSymbol { "dir": ".", "oldName": "List", "newName": "ListTasks", "dryRun": true }
`

// ListImportsDesc describes the listImports tool.
const ListImportsDesc = `
Lists all imported packages in Go source files under the given directory.

Use when:
- You want to inspect dependencies at the file level.
- Useful for dependency analysis or cleanup of unused imports.

Notes:
- Optionally filter by package path as reported by 'go list' (for example, 'go-navigator/internal/tools').

Example:
listImports { "dir": ".", "package": "go-navigator/internal/tools" }
`

// ListInterfacesDesc describes the listInterfaces tool.
const ListInterfacesDesc = `
Lists all interfaces and their methods for dependency analysis or mocking.

Use when:
- You want to identify abstractions or create mocks.
- Helps locate key interfaces for testing or refactoring.

Notes:
- Optionally filter by package path as reported by 'go list' (for example, 'go-navigator/internal/tools').

Example:
listInterfaces { "dir": ".", "package": "go-navigator/internal/tools" }
`

// GetComplexityReportDesc describes the getComplexityReport tool.
const GetComplexityReportDesc = `
Analyzes function metrics: lines of code, nesting depth, and cyclomatic complexity.

Use when:
- You want to find complex functions to refactor or simplify.
- Great for technical debt analysis and code reviews.
- Optionally scope analysis to a specific package path (e.g. "go-navigator/internal/tools").

Example:
getComplexityReport { "dir": ".", "package": "go-navigator/internal/tools" }
`

// GetDeadCodeReportDesc describes the getDeadCodeReport tool.
const GetDeadCodeReportDesc = `
Finds unused functions, variables, constants, and types within the Go project.

Use when:
- You want to identify dead or unreferenced code.
- Useful before cleanup or preparing a release.
- Need per-kind and per-package counts with optional result limiting.
- Optionally restrict scanning to a specific package path (e.g. "go-navigator/internal/tools").

Example:
getDeadCodeReport { "dir": ".", "package": "go-navigator/internal/tools", "limit": 10 }
`

// GetDependencyGraphDesc describes the getDependencyGraph tool.
const GetDependencyGraphDesc = `
Builds a graph of dependencies between internal packages (imports, cycles, fan-in/fan-out).

Use when:
- You need to understand coupling between packages.
- Useful for architecture review and dependency graph generation.
- Optionally scope analysis to a specific package path (e.g. "go-navigator/internal/tools").

Example:
getDependencyGraph { "dir": ".", "package": "go-navigator/internal/tools" }
`

// GetImplementationsDesc describes the getImplementations tool.
const GetImplementationsDesc = `
Shows which concrete types implement interfaces (and vice versa).

Use when:
- You need to locate all implementations of an interface.
- Useful for refactoring or interface-based testing.

Example:
getImplementations { "dir": ".", "name": "Repository" }
`

// GetMetricsSummaryDesc describes the getMetricsSummary tool.
const GetMetricsSummaryDesc = `
Aggregates general project metrics: package/struct/interface counts, average cyclomatic complexity, and unused code ratios.

Use when:
- You want a quick summary of codebase health.
- Useful for dashboards, reports, or CI metrics.
- Optionally scope aggregation to a specific package path (e.g. "go-navigator/internal/tools").

Example:
getMetricsSummary { "dir": ".", "package": "go-navigator/internal/tools" }
`

// RewriteAstDesc describes the rewriteAst tool.
const RewriteAstDesc = `
Performs semantic AST rewriting with pattern matching.

Use when:
- You need to apply controlled code transformations (e.g. convert pkg.Func(x) → x.Method()).
- Supports dry-run preview for safe testing.

Example:
rewriteAst { "dir": ".", "find": "fmt.Println(x)", "replace": "log.Print(x)", "dryRun": true }
`

// GetFunctionSourceDesc describes the getFunctionSource tool.
const GetFunctionSourceDesc = `
Returns the full source code and metadata of a Go function or method by name.

Use when:
- You want to inspect or extract a function implementation.
- Helpful for code review, documentation, or AI-assisted editing.

Example:
getFunctionSource { "dir": ".", "name": "TaskService.List" }
`

// GetFileInfoDesc describes the getFileInfo tool.
const GetFileInfoDesc = `
Reads and analyzes a Go source file, returning its package name, imports, and declared symbols.
Optionally includes source code and comments based on the provided options.

Use when:
- You need to inspect or analyze a specific Go source file.
- You want to extract functions, structs, interfaces, constants, or variables from the file.
- You need the file's source or symbol metadata for further analysis or refactoring.

Supported fields:
- options.withSource: include full source code
- options.withComments: include symbol comments
- options.includeFunctionBodies: include function bodies (limited by functionBodyLimit)
- filter.symbolKinds: restrict to specific symbol types (func, struct, interface, var, const)
- filter.nameContains: filter by substring match
- filter.exportedOnly: include only exported symbols

Example:
getFileInfo {
  "dir": ".",
  "file": "internal/tools/server.go",
  "options": { "withSource": true, "withComments": true },
  "filter": { "symbolKinds": ["func"], "exportedOnly": true }
}
`

// GetStructInfoDesc describes the getStructInfo tool.
const GetStructInfoDesc = `
Returns the declaration of a Go struct, including its fields, tags, comments, and optionally its methods.

Use when:
- You want to inspect data models or generate documentation.
- Supports IncludeMethods to list associated methods.

Example:
getStructInfo { "dir": ".", "name": "User", "includeMethods": true }
`

const GetProjectSchemaDesc = `
Aggregates a full structural schema of a Go project including:
- module name and Go version
- packages and their imports
- structs, interfaces, and functions
- external dependencies and inter-package dependency graph

🪶 Use when:
- You need a high-level overview of a Go module
- You want to visualize or analyze package relationships
- Supports configurable detail levels via 'depth' parameter:
  * 'summary': minimal analysis with basic project metadata and package counts
  * 'standard': full analysis including packages, symbols, interfaces, and dependencies (default)
  * 'deep': extended analysis with additional detailed information (future extensibility)

💡 Example:
getProjectSchema { "dir": ".", "depth": "standard" }
`
