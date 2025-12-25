package tools

// Centralized tool descriptions for Go Navigator MCP server.
// Each description is concise, structured, and suitable for AI and human use.

// ListPackagesDesc describes the listPackages tool.
const ListPackagesDesc = `
List Go packages under a directory.
Example: listPackages { "dir": "." }
`

// ListSymbolsDesc describes the listSymbols tool.
const ListSymbolsDesc = `
List functions, structs, interfaces, and methods in a package (go list path).
Example: listSymbols { "dir": ".", "package": "go-navigator/internal/tools" }
`

// GetDefinitionsDesc describes the getDefinitions tool.
const GetDefinitionsDesc = `
Find definition sites for an identifier; grouped by file, supports limit/offset.
Example: getDefinitions { "dir": ".", "ident": "TaskService" }
`

// GetReferencesDesc describes the getReferences tool.
const GetReferencesDesc = `
Find usages of an identifier; grouped by file, supports limit/offset.
Example: getReferences { "dir": ".", "ident": "TaskService" }
`

// GetSymbolContextDesc describes the getSymbolContext tool.
const GetSymbolContextDesc = `
Focused context bundle: definition, key usages, direct imports.
Example: getSymbolContext { "dir": ".", "ident": "DoSomething", "kind": "func" }
`

// RenameSymbolDesc describes the renameSymbol tool.
const RenameSymbolDesc = `
Scope-aware rename with collision detection; use dryRun first.
Example: renameSymbol { "dir": ".", "oldName": "List", "newName": "ListTasks", "dryRun": true }
`

// ListImportsDesc describes the listImports tool.
const ListImportsDesc = `
List imports per file; optional package filter (go list path).
Example: listImports { "dir": ".", "package": "go-navigator/internal/tools" }
`

// ListInterfacesDesc describes the listInterfaces tool.
const ListInterfacesDesc = `
List interfaces and methods; optional package filter (go list path).
Example: listInterfaces { "dir": ".", "package": "go-navigator/internal/tools" }
`

// GetComplexityReportDesc describes the getComplexityReport tool.
const GetComplexityReportDesc = `
Function metrics: LoC, nesting depth, cyclomatic complexity; optional package filter.
Example: getComplexityReport { "dir": ".", "package": "go-navigator/internal/tools" }
`

// GetDeadCodeReportDesc describes the getDeadCodeReport tool.
const GetDeadCodeReportDesc = `
Unused symbols report; optional package filter and limit.
Example: getDeadCodeReport { "dir": ".", "package": "go-navigator/internal/tools", "limit": 10 }
`

// GetDependencyGraphDesc describes the getDependencyGraph tool.
const GetDependencyGraphDesc = `
Internal package dependency graph; optional package filter.
Example: getDependencyGraph { "dir": ".", "package": "go-navigator/internal/tools" }
`

// GetImplementationsDesc describes the getImplementations tool.
const GetImplementationsDesc = `
Interface <-> concrete type implementations.
Example: getImplementations { "dir": ".", "name": "Repository" }
`

// GetMetricsSummaryDesc describes the getMetricsSummary tool.
const GetMetricsSummaryDesc = `
Aggregated metrics (counts, avg complexity, unused ratios); optional package filter.
Example: getMetricsSummary { "dir": ".", "package": "go-navigator/internal/tools" }
`

// RewriteAstDesc describes the rewriteAst tool.
const RewriteAstDesc = `
Semantic AST rewrite with pattern matching; supports dryRun.
Example: rewriteAst { "dir": ".", "find": "fmt.Println(x)", "replace": "log.Print(x)", "dryRun": true }
`

// GetFunctionSourceDesc describes the getFunctionSource tool.
const GetFunctionSourceDesc = `
Return function/method source + metadata by name.
Example: getFunctionSource { "dir": ".", "name": "TaskService.List" }
`

// GetFileInfoDesc describes the getFileInfo tool.
const GetFileInfoDesc = `
Read file metadata; optional source/comments/bodies via options/filter.
Example: getFileInfo { "dir": ".", "file": "internal/tools/server.go", "options": { "withSource": true } }
`

// GetStructInfoDesc describes the getStructInfo tool.
const GetStructInfoDesc = `
Return a struct declaration; includeMethods lists associated methods.
Example: getStructInfo { "dir": ".", "name": "User", "includeMethods": true }
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
