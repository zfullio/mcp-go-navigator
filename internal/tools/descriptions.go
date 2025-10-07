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
- Useful before using findDefinitions or renameSymbol.

Example:
listSymbols { "dir": ".", "package": "./internal/tools" }
`

// FindDefinitionsDesc describes the findDefinitions tool.
const FindDefinitionsDesc = `
Locates where a symbol is defined (type, func, var, const).

Use when:
- You need to find where a symbol is declared or defined.
- Ideal for navigation, documentation, or analysis.

Example:
findDefinitions { "dir": ".", "ident": "TaskService" }
`

// FindReferencesDesc describes the findReferences tool.
const FindReferencesDesc = `
Finds all references and usages of an identifier using go/types semantic analysis.

Use when:
- You want to check where a symbol is used before renaming or deleting it.
- Ideal companion to findDefinitions.

Example:
findReferences { "dir": ".", "ident": "TaskService" }
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

Example:
listImports { "dir": "." }
`

// ListInterfacesDesc describes the listInterfaces tool.
const ListInterfacesDesc = `
Lists all interfaces and their methods for dependency analysis or mocking.

Use when:
- You want to identify abstractions or create mocks.
- Helps locate key interfaces for testing or refactoring.

Example:
listInterfaces { "dir": "." }
`

// AnalyzeComplexityDesc describes the analyzeComplexity tool.
const AnalyzeComplexityDesc = `
Analyzes function metrics: lines of code, nesting depth, and cyclomatic complexity.

Use when:
- You want to find complex functions to refactor or simplify.
- Great for technical debt analysis and code reviews.

Example:
analyzeComplexity { "dir": "." }
`

// DeadCodeDesc describes the deadCode tool.
const DeadCodeDesc = `
Finds unused functions, variables, constants, and types within the Go project.

Use when:
- You want to identify dead or unreferenced code.
- Useful before cleanup or preparing a release.

Example:
deadCode { "dir": "." }
`

// AnalyzeDependenciesDesc describes the analyzeDependencies tool.
const AnalyzeDependenciesDesc = `
Builds a graph of dependencies between internal packages (imports, cycles, fan-in/fan-out).

Use when:
- You need to understand coupling between packages.
- Useful for architecture review and dependency graph generation.

Example:
analyzeDependencies { "dir": "." }
`

// FindImplementationsDesc describes the findImplementations tool.
const FindImplementationsDesc = `
Shows which concrete types implement interfaces (and vice versa).

Use when:
- You need to locate all implementations of an interface.
- Useful for refactoring or interface-based testing.

Example:
findImplementations { "dir": ".", "name": "Repository" }
`

// MetricsSummaryDesc describes the metricsSummary tool.
const MetricsSummaryDesc = `
Aggregates general project metrics: package/struct/interface counts, average cyclomatic complexity, and unused code ratios.

Use when:
- You want a quick summary of codebase health.
- Useful for dashboards, reports, or CI metrics.

Example:
metricsSummary { "dir": "." }
`

// ASTRewriteDesc describes the astRewrite tool.
const ASTRewriteDesc = `
Performs semantic AST rewriting with pattern matching.

Use when:
- You need to apply controlled code transformations (e.g. convert pkg.Func(x) → x.Method()).
- Supports dry-run preview for safe testing.

Example:
astRewrite { "dir": ".", "find": "fmt.Println(x)", "replace": "log.Print(x)", "dryRun": true }
`

// ReadFuncDesc describes the readFunc tool.
const ReadFuncDesc = `
Returns the full source code and metadata of a Go function or method by name.

Use when:
- You want to inspect or extract a function implementation.
- Helpful for code review, documentation, or AI-assisted editing.

Example:
readFunc { "dir": ".", "name": "TaskService.List" }
`

// ReadFileDesc describes the readFile tool.
const ReadFileDesc = `
Reads a Go source file and returns its package, imports, declared symbols, line count, and optionally full source code.

Use when:
- You need to examine the contents or metadata of a file.
- Supports "raw", "summary", and "ast" modes.

Example:
readFile { "dir": ".", "file": "internal/tools/sample.go", "mode": "summary" }
`

// ReadStructDesc describes the readStruct tool.
const ReadStructDesc = `
Returns the declaration of a Go struct, including its fields, tags, comments, and optionally its methods.

Use when:
- You want to inspect data models or generate documentation.
- Supports IncludeMethods to list associated methods.

Example:
readStruct { "dir": ".", "name": "User", "includeMethods": true }
`

const ProjectSchemaDesc = `
Aggregates a full structural schema of a Go project including:
- module name and Go version
- packages and their imports
- structs, interfaces, and functions
- external dependencies and inter-package dependency graph

🪶 Use when:
- You need a high-level overview of a Go module
- You want to visualize or analyze package relationships

💡 Example:
projectSchema { "dir": ".", "depth": "standard" }
`
