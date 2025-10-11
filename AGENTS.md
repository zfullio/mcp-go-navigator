# Go-Navigator-MCP Agent Handbook

## Mission
- `go-help-mcp` exposes the **Go Navigator** MCP server (v1.4.0) for semantic Go code analysis and refactoring.
- The server communicates over stdio and registers all tools from `internal/tools`.
- Primary goals for agents: prefer structured MCP tools over ad-hoc parsing, minimise token usage by relying on grouped outputs, keep responses actionable.

## Repository Layout
```
/home/viktor-d/Programming/MyProjects/go-help-mcp/
├── AGENTS.md                 # this handbook
├── LICENSE
├── README.md                 # high-level overview
├── cmd/
│   └── go-navigator/
│       └── main.go           # MCP server entry point
├── internal/
│   └── tools/
│       ├── analyzers.go      # metrics, dead code, dependency graph tools
│       ├── analyzers_test.go # tests for analyzers.go
│       ├── cache.go          # package/file caches shared across tools
│       ├── descriptions.go   # tool metadata used during registration
│       ├── finders.go        # definitions/references/implementations lookups
│       ├── finders_test.go   # tests for finders.go
│       ├── health.go         # HealthCheck()
│       ├── helpers.go        # shared AST utilities, diff helpers
│       ├── listers.go        # list tools (packages, symbols, imports, interfaces)
│       ├── listers_test.go   # tests for listers.go
│       ├── logging.go        # structured logging helpers
│       ├── readers.go        # readGoFile/readFunc/readStruct implementations
│       ├── readers_test.go   # tests for readers.go
│       ├── refactorers.go    # renameSymbol, astRewrite and other mutating flows
│       ├── refactorers_test.go # tests for refactorers.go
│       ├── types.go          # JSON schemas for inputs/outputs
│       └── testdata/sample/  # fixtures (bar.go, foo.go, complex.go, empty_interface.go,
│                             #             dead.go, store.go, print.go)
├── go.mod (go 1.25)
└── go.sum
```

## MCP Tool Catalog
**Project overview**
- `listPackages` — discover packages under `dir`.
- `metricsSummary` — aggregate counts (packages/interfaces), average cyclomatic complexity, unused symbol ratios (supports package filter).
- `analyzeComplexity` — function metrics (cyclomatic, nesting, LoC) with optional package filter.
- `analyzeDependencies` — dependency graph with fan-in/fan-out and cycle detection (supports package filter).
- `projectSchema` — aggregate full structural metadata of a Go module including packages, symbols, interfaces, imports, and dependency graph. Supports configurable detail levels via `depth` parameter (summary, standard, or deep).

**Structure & navigation**
- `listSymbols` — returns `groupedSymbols[{package, files[{file, symbols[]}]}]` (no flat list).
- `listImports` — imports grouped per file (`imports[{file, imports[]}]`).
- `listInterfaces` — interfaces grouped per package (`interfaces[{package, interfaces[]}]`).
- `findDefinitions` — definition sites for identifiers.
- `findReferences` — all usages with optional `file` / `kind` filters.
- `findImplementations` — interface ↔ concrete type relationships.

**Source inspection**
- `readGoFile` — package metadata, imports, declared symbols, optional full `source` (set `withSource=true`).
- `readFunc` — body and metadata of a function/method by name.
- `readStruct` — struct declaration (optionally include associated methods).

**Quality & refactoring**
- `analyzeComplexity` — function metrics grouped by file.
- `deadCode` — unused symbols (extend scope with `includeExported=true`, supports package filter).
- `renameSymbol` — safe rename with dry-run diff (`preview=true`) and collision reports.
- `astRewrite` — pattern-driven AST transformations (start with `dryRun=true`).

## Response & Token Guidance
- Clients must consume grouped outputs only; flat fields were removed to cut token usage.
- Prefer summarised counts or key findings; include raw entries only when essential.
- Unified diffs (`renameSymbol` outputs) are already condensed — do not expand them.
- JSON schemas use short keys (`dir`, `package`, `ident`); omit optional fields unless required by the tool.

## Build & Test Basics
- Build: `go build -o go-navigator ./cmd/go-navigator`.
- Recommended test run: `GOCACHE=$(pwd)/.gocache go test ./...` (delete `.gocache/` afterwards if needed).
- `*_test.go` (e.g., `listers_test.go`, `finders_test.go`, `refactorers_test.go`): Decomposed test suites for each tool category: discovery (`listPackages`), navigation (`listSymbols`, `listImports`, `listInterfaces`, `findDefinitions`, `findReferences`, `findImplementations`), analysis (`analyzeComplexity`, `metricsSummary`, `deadCode`, `analyzeDependencies`), source readers (`readGoFile`, `readFunc`, `readStruct`), refactoring (`renameSymbol`, `astRewrite`), and `HealthCheck`. This structure allows for targeted testing of individual functionalities.

## Recommended Agent Flow
1. Start with `projectSchema` using configurable `depth` parameter (summary, standard, or deep) to get comprehensive structural metadata of the Go module including packages, symbols, interfaces, imports, and dependency graph.
2. Use `listPackages` to explore the overall package structure if needed.
3. Examine symbols and interfaces with `listSymbols`/`listInterfaces` for detailed architecture understanding.
4. Analyze dependencies and imports via `analyzeDependencies` and `listImports` to understand the project's topology.
5. Assess code quality with `analyzeComplexity`, `metricsSummary`, and `deadCode` to identify hotspots and issues.
6. Navigate to specific elements using `findDefinitions` and `findReferences` for detailed investigation.
7. Use `findImplementations` to understand type hierarchies and interface relationships.
8. For refactoring proposals, execute `renameSymbol` with `preview=true` and review the diff + collision report.
9. When detailed source context is needed, use `readGoFile`/`readFunc`/`readStruct` to get specific code elements.

## Operational Notes
- `helpers.go` still hosts the heavy AST comparison utilities (`compareASTNodes`), while `refactorers.go` carries the complex rename pipeline; treat both as prime refactor targets when feasible.
- MCP clients must already handle grouped schemas for imports/interfaces/symbols; do not reintroduce legacy flat outputs.
- Module targets Go 1.25 — older toolchains may fail.
- When extending tests, add new fixtures under `internal/tools/testdata/sample/`; existing files cover edge cases (empty interfaces, dead code, complex control flow).
