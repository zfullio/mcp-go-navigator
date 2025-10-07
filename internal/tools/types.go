package tools

// ------------------ list packages ------------------

// ListPackagesInput contains input data for the ListPackages tool.
type ListPackagesInput struct {
	// Dir - root directory to scan for Go packages
	Dir string `json:"dir" jsonschema:"Root directory to scan for Go packages"`
}

// ListPackagesOutput contains results from the ListPackages tool.
type ListPackagesOutput struct {
	// Packages - list of discovered Go packages
	Packages []string `json:"packages" jsonschema:"List of discovered Go package import paths"`
}

// ------------------ list symbols ------------------

// ListSymbolsInput contains input data for the ListSymbols tool.
type ListSymbolsInput struct {
	// Dir - root directory of the Go module
	Dir string `json:"dir" jsonschema:"Root directory of the Go module"`
	// Package - path to package to find symbols in
	Package string `json:"package" jsonschema:"Package path to inspect for symbols"`
}

// Symbol represents a symbol (function, struct, interface, etc.) in Go code.
type Symbol struct {
	// Kind - symbol type (func, struct, interface, method, etc.)
	Kind string `json:"kind" jsonschema:"Symbol type (func, struct, interface, method, etc.)"`
	// Name - symbol name
	Name string `json:"name" jsonschema:"Symbol name"`
	// Package - package where the symbol is defined
	Package string `json:"package" jsonschema:"Package where the symbol is defined"`
	// File - file where the symbol is defined
	File string `json:"file" jsonschema:"File where the symbol is defined"`
	// Line - line number in the file
	Line int `json:"line" jsonschema:"Line number in the file"`
	// Exported - true if the symbol is exported (starts with capital letter)
	Exported bool `json:"exported" jsonschema:"True if the symbol is exported (starts with capital letter)"`
}

// SymbolGroupByFile represents symbols grouped by file within a package.
type SymbolGroupByFile struct {
	// File - file where the symbols are defined
	File string `json:"file" jsonschema:"File where the symbols are defined"`
	// Symbols - list of symbols in this file
	Symbols []SymbolInfo `json:"symbols" jsonschema:"List of symbols in this file"`
}

// SymbolGroupByPackage represents files and symbols grouped by package.
type SymbolGroupByPackage struct {
	// Package - package where the symbols are defined
	Package string `json:"package" jsonschema:"Package where the symbols are defined"`
	// Files - list of files with symbols in this package
	Files []SymbolGroupByFile `json:"files" jsonschema:"List of files with symbols in this package"`
}

// SymbolInfo represents the core information about a symbol (without package/file details, since they're grouped).
type SymbolInfo struct {
	// Kind - symbol type (func, struct, interface, method, etc.)
	Kind string `json:"kind" jsonschema:"Symbol type (func, struct, interface, method, etc.)"`
	// Name - symbol name
	Name string `json:"name" jsonschema:"Symbol name"`
	// Line - line number in the file
	Line int `json:"line" jsonschema:"Line number in the file"`
	// Exported - true if the symbol is exported (starts with capital letter)
	Exported bool `json:"exported" jsonschema:"True if the symbol is exported (starts with capital letter)"`
}

// ListSymbolsOutput contains results from the ListSymbols tool.
type ListSymbolsOutput struct {
	// GroupedSymbols - symbols found, grouped by package and file (alternative format for token efficiency)
	GroupedSymbols []SymbolGroupByPackage `json:"groupedSymbols,omitempty" jsonschema:"Symbols grouped by package and file"`
}

// ------------------ find references ------------------

// FindReferencesInput contains input data for the FindReferences tool.
type FindReferencesInput struct {
	// Dir - root directory of the Go module
	Dir string `json:"dir" jsonschema:"Root directory of the Go module"`
	// Ident - name of the symbol to find references for
	Ident string `json:"ident" jsonschema:"Name of the symbol to find references for"`
	// File - optional relative file path to restrict the search
	File string `json:"file,omitempty" jsonschema:"Optional relative file path to restrict the search"`
	// Kind - filter by symbol type (e.g. func, type, var, const)
	Kind string `json:"kind,omitempty" jsonschema:"Filter by symbol kind (e.g. func, type, var, const)"`
	// Limit - maximum number of references to return (0 means no limit)
	Limit int `json:"limit,omitempty" jsonschema:"Maximum number of references to return (0 means no limit)"`
	// Offset - number of references to skip before returning results
	Offset int `json:"offset,omitempty" jsonschema:"Number of references to skip before returning results"`
}

// ReferenceEntry represents a reference occurrence within a file.
type ReferenceEntry struct {
	// Line - line number of the reference
	Line int `json:"line" jsonschema:"Line number of the reference"`
	// Snippet - code context showing the reference usage
	Snippet string `json:"snippet" jsonschema:"Code context showing the reference usage"`
}

// ReferenceGroup groups references by file.
type ReferenceGroup struct {
	// File - relative path to the file containing the references
	File string `json:"file" jsonschema:"Relative path to the file containing the references"`
	// References - list of reference occurrences within the file
	References []ReferenceEntry `json:"references" jsonschema:"List of reference occurrences within the file"`
}

// FindReferencesOutput contains results from the FindReferences tool.
type FindReferencesOutput struct {
	// Total - total number of references that were found (before pagination)
	Total int `json:"total" jsonschema:"Total number of references found before pagination"`
	// Offset - number of references skipped before returning results
	Offset int `json:"offset" jsonschema:"Number of references skipped before returning results"`
	// Limit - maximum number of references returned (0 when no limit was applied)
	Limit int `json:"limit,omitempty" jsonschema:"Maximum number of references returned (0 when no limit was applied)"`
	// Groups - references grouped by file
	Groups []ReferenceGroup `json:"groups,omitempty" jsonschema:"References grouped by file"`
}

// ------------------ find definitions ------------------

// FindDefinitionsInput contains input data for the FindDefinitions tool.
type FindDefinitionsInput struct {
	// Dir - root directory of the Go module
	Dir string `json:"dir" jsonschema:"Root directory of the Go module"`
	// Ident - name of the symbol to locate its definition
	Ident string `json:"ident" jsonschema:"Name of the symbol to locate its definition"`
	// File - optional relative file path to restrict the search
	File string `json:"file,omitempty" jsonschema:"Optional relative file path to restrict the search"`
	// Kind - filter by symbol type (e.g. func, type, var, const)
	Kind string `json:"kind,omitempty" jsonschema:"Filter by symbol kind (e.g. func, type, var, const)"`
	// Limit - maximum number of definitions to return (0 means no limit)
	Limit int `json:"limit,omitempty" jsonschema:"Maximum number of definitions to return (0 means no limit)"`
	// Offset - number of definitions to skip before returning results
	Offset int `json:"offset,omitempty" jsonschema:"Number of definitions to skip before returning results"`
}

// DefinitionEntry represents a definition occurrence within a file.
type DefinitionEntry struct {
	// Line - line number of the definition
	Line int `json:"line" jsonschema:"Line number of the definition"`
	// Snippet - code snippet showing the definition line
	Snippet string `json:"snippet" jsonschema:"Code snippet showing the definition line"`
}

// DefinitionGroup groups symbol definitions by file.
type DefinitionGroup struct {
	// File - relative path to the file where the symbol is defined
	File string `json:"file" jsonschema:"Relative path to the file where the symbol is defined"`
	// Definitions - list of definition occurrences within the file
	Definitions []DefinitionEntry `json:"definitions" jsonschema:"List of definition occurrences within the file"`
}

// FindDefinitionsOutput contains results from the FindDefinitions tool.
type FindDefinitionsOutput struct {
	// Total - total number of definitions that were found (before pagination)
	Total int `json:"total" jsonschema:"Total number of definitions found before pagination"`
	// Offset - number of definitions skipped before returning results
	Offset int `json:"offset" jsonschema:"Number of definitions skipped before returning results"`
	// Limit - maximum number of definitions returned (0 when no limit was applied)
	Limit int `json:"limit,omitempty" jsonschema:"Maximum number of definitions returned (0 when no limit was applied)"`
	// Groups - definitions grouped by file
	Groups []DefinitionGroup `json:"groups,omitempty" jsonschema:"Definitions grouped by file"`
}

// ------------------ list imports ------------------

// ListImportsInput contains input data for the ListImports tool.
type ListImportsInput struct {
	// Dir - root directory to scan for Go files
	Dir string `json:"dir" jsonschema:"Root directory to scan for Go files"`
	// Package - optional package path to restrict results
	Package string `json:"package,omitempty" jsonschema:"Optional Go package path to restrict the scan"`
}

// Import represents an import of a package in a Go file.
type Import struct {
	// Path - imported package path
	Path string `json:"path" jsonschema:"Imported package path"`
	// File - file where the import is declared
	File string `json:"file" jsonschema:"File where the import is declared"`
	// Line - line number of the import statement
	Line int `json:"line" jsonschema:"Line number of the import statement"`
}

// ImportInfo stores import data without repeating the file.
type ImportInfo struct {
	// Path - imported package path
	Path string `json:"path" jsonschema:"Imported package path"`
	// Line - line number of the import statement
	Line int `json:"line" jsonschema:"Line number of the import statement"`
}

// ImportGroupByFile groups imports by file.
type ImportGroupByFile struct {
	// File - file that declares the imports
	File string `json:"file" jsonschema:"File that declares the imports"`
	// Imports - list of imports in the file
	Imports []ImportInfo `json:"imports" jsonschema:"List of imports declared in the file"`
}

// ListImportsOutput contains results from the ListImports tool.
type ListImportsOutput struct {
	// Imports - imports grouped by file (token efficiency)
	Imports []ImportGroupByFile `json:"imports,omitempty" jsonschema:"Imports grouped by file"`
}

// ------------------ list interfaces ------------------

// ListInterfacesInput contains input data for the ListInterfaces tool.
type ListInterfacesInput struct {
	// Dir - root directory to scan for Go files
	Dir string `json:"dir" jsonschema:"Root directory to scan for Go files"`
	// Package - optional package path to restrict results
	Package string `json:"package,omitempty" jsonschema:"Optional Go package path to restrict the scan"`
}

// InterfaceMethod represents an interface method.
type InterfaceMethod struct {
	// Name - method name
	Name string `json:"name" jsonschema:"Method name"`
	// Line - line number of the method
	Line int `json:"line" jsonschema:"Line number of the method"`
}

// InterfaceInfo represents information about an interface.
type InterfaceInfo struct {
	// Name - interface name
	Name string `json:"name" jsonschema:"Interface name"`
	// File - file where the interface is defined
	File string `json:"file" jsonschema:"File where the interface is defined"`
	// Line - line number of the interface declaration
	Line int `json:"line" jsonschema:"Line number of the interface declaration"`
	// Methods - list of methods defined in the interface
	Methods []InterfaceMethod `json:"methods" jsonschema:"List of methods defined in the interface"`
}

// InterfaceGroupByPackage groups interfaces by package.
type InterfaceGroupByPackage struct {
	// Package - package where interfaces are defined
	Package string `json:"package" jsonschema:"Package where interfaces are defined"`
	// Interfaces - list of interfaces in the package
	Interfaces []InterfaceInfo `json:"interfaces" jsonschema:"Interfaces declared within the package"`
}

// ListInterfacesOutput contains results from the ListInterfaces tool.
type ListInterfacesOutput struct {
	// Interfaces - interfaces grouped by package
	Interfaces []InterfaceGroupByPackage `json:"interfaces,omitempty" jsonschema:"Interfaces grouped by package"`
}

// ------------------ analyze complexity ------------------

// AnalyzeComplexityInput contains input data for the AnalyzeComplexity tool.
type AnalyzeComplexityInput struct {
	// Dir - root directory to scan for Go files
	Dir string `json:"dir" jsonschema:"Root directory to scan for Go files"`
	// Package - optional package path to restrict results
	Package string `json:"package,omitempty" jsonschema:"Optional Go package path to restrict the scan"`
}

// FunctionComplexityGroupByFile represents symbols grouped by file within a package.
type FunctionComplexityGroupByFile struct {
	// File - file where the function is defined
	File string `json:"file" jsonschema:"File where the symbols are defined"`
	// Functions - list of functions in this file
	Functions []FunctionComplexityInfo `json:"functions" jsonschema:"Calculated complexity metrics for all functions"`
}

// FunctionComplexity represents function complexity metrics.
type FunctionComplexity struct {
	// Name - function name
	Name string `json:"name" jsonschema:"Function name"`
	// File - file where the function is defined
	File string `json:"file" jsonschema:"File where the function is defined"`
	// Line - line number of the function
	Line int `json:"line" jsonschema:"Line number of the function"`
	// Lines - total number of lines in the function
	Lines int `json:"lines" jsonschema:"Total number of lines in the function"`
	// Nesting - maximum nesting depth
	Nesting int `json:"nesting" jsonschema:"Maximum nesting depth"`
	// Cyclomatic - cyclomatic complexity
	Cyclomatic int `json:"cyclomatic" jsonschema:"Cyclomatic complexity value"`
}

type FunctionComplexityInfo struct {
	// Name - function name
	Name string `json:"name" jsonschema:"Function name"`
	// Line - line number of the function
	Line int `json:"line" jsonschema:"Line number of the function"`
	// Lines - total number of lines in the function
	Lines int `json:"lines" jsonschema:"Total number of lines in the function"`
	// Nesting - maximum nesting depth
	Nesting int `json:"nesting" jsonschema:"Maximum nesting depth"`
	// Cyclomatic - cyclomatic complexity
	Cyclomatic int `json:"cyclomatic" jsonschema:"Cyclomatic complexity value"`
}

// AnalyzeComplexityOutput contains results from the AnalyzeComplexity tool.
type AnalyzeComplexityOutput struct {
	// Functions - calculated complexity metrics for all functions
	Functions []FunctionComplexityGroupByFile `json:"functions" jsonschema:"Calculated complexity metrics for functions"`
}

// ------------------ dead code ------------------

// DeadCodeInput contains input data for the DeadCode tool.
type DeadCodeInput struct {
	// Dir - root directory to scan for unused symbols
	Dir string `json:"dir" jsonschema:"Root directory to scan for unused symbols"`
	// IncludeExported - if true, includes exported symbols that are unused
	IncludeExported bool `json:"includeExported,omitempty" jsonschema:"If true, include exported symbols that are unused"`
	// Limit - optional maximum number of unused symbols to return (0 means no limit)
	Limit int `json:"limit,omitempty" jsonschema:"Optional maximum number of unused symbols to include in the response (0 means no limit)"`
	// Package - optional package path to restrict the scan
	Package string `json:"package,omitempty" jsonschema:"Optional Go package path to restrict the scan"`
}

// DeadSymbol represents an unused symbol in Go code.
type DeadSymbol struct {
	// Name - symbol name
	Name string `json:"name" jsonschema:"Symbol name"`
	// Kind - symbol kind (func, var, const, type)
	Kind string `json:"kind" jsonschema:"Symbol kind (func, var, const, type)"`
	// File - file where the unused symbol is declared
	File string `json:"file" jsonschema:"File where the unused symbol is declared"`
	// Line - line number of the symbol
	Line int `json:"line" jsonschema:"Line number of the symbol"`
	// IsExported - true if the symbol is exported (starts with capital letter)
	IsExported bool `json:"isExported" jsonschema:"True if the symbol is exported (starts with capital letter)"`
	// Package - package where the symbol is defined
	Package string `json:"package" jsonschema:"Package where the symbol is defined"`
}

// DeadCodeOutput contains results from the DeadCode tool.
type DeadCodeOutput struct {
	// Unused - list of unused or dead code symbols
	Unused []DeadSymbol `json:"unused" jsonschema:"List of unused or dead code symbols"`
	// TotalCount - total number of unused symbols found
	TotalCount int `json:"totalCount" jsonschema:"Total number of unused symbols found"`
	// ExportedCount - number of unused exported symbols
	ExportedCount int `json:"exportedCount" jsonschema:"Number of exported symbols that are unused"`
	// ByPackage - count of unused symbols grouped by package
	ByPackage map[string]int `json:"byPackage" jsonschema:"Count of unused symbols grouped by package"`
	// ByKind - count of unused symbols grouped by symbol kind (func, var, const, type)
	ByKind map[string]int `json:"byKind,omitempty" jsonschema:"Count of unused symbols grouped by symbol kind (func, var, const, type)"`
	// HasMore - true when the response was limited and more results are available
	HasMore bool `json:"hasMore,omitempty" jsonschema:"True if more unused symbols exist beyond the returned list"`
}

// ------------------ rename symbol ------------------

// RenameSymbolInput contains input data for the RenameSymbol tool.
type RenameSymbolInput struct {
	// Dir - root directory of the Go module
	Dir string `json:"dir" jsonschema:"Root directory of the Go module"`
	// OldName - current symbol name to rename; supports format 'TypeName.MethodName' for methods
	OldName string `json:"oldName" jsonschema:"Current symbol name to rename; supports format 'TypeName.MethodName' for methods"`
	// NewName - new symbol name to apply
	NewName string `json:"newName" jsonschema:"New symbol name to apply"`
	// Kind - symbol kind: func, var, const, type, package
	Kind string `json:"kind,omitempty" jsonschema:"Symbol kind: func, var, const, type, package"`
	// DryRun - if true, returns only a preview of changes without writing files
	DryRun bool `json:"dryRun,omitempty" jsonschema:"If true, only return a diff preview without writing files"`
}

// FileDiff represents delta of changes in a file.
type FileDiff struct {
	// Path - file path where changes occurred
	Path string `json:"path" jsonschema:"File path where changes occurred"`
	// Diff - unified diff showing changes
	Diff string `json:"diff" jsonschema:"Unified diff showing changes"`
}

// RenameSymbolOutput contains results from the RenameSymbol tool.
type RenameSymbolOutput struct {
	// ChangedFiles - list of modified files
	ChangedFiles []string `json:"changedFiles" jsonschema:"List of modified files"`
	// Diffs - diff results if dry run was used
	Diffs []FileDiff `json:"diffs,omitempty" jsonschema:"Diff results if dry run was used"`
	// Collisions - list of name conflicts preventing rename
	Collisions []string `json:"collisions,omitempty" jsonschema:"List of name conflicts preventing rename"`
}

// ------------------ analyze dependencies ------------------.

// AnalyzeDependenciesInput contains input data for the AnalyzeDependencies tool.
type AnalyzeDependenciesInput struct {
	// Dir - root directory to scan for package dependencies
	Dir string `json:"dir" jsonschema:"Root directory to scan for package dependencies"`
	// Package - optional package path to restrict results
	Package string `json:"package,omitempty" jsonschema:"Optional Go package path to restrict the scan"`
}

// PackageDependency represents information about package dependencies.
type PackageDependency struct {
	// Package - package path
	Package string `json:"package" jsonschema:"Package path"`
	// Imports - list of imported packages
	Imports []string `json:"imports" jsonschema:"List of imported packages"`
	// FanIn - number of other packages that import this package
	FanIn int `json:"fanIn" jsonschema:"Number of other packages that import this package"`
	// FanOut - number of packages this package imports
	FanOut int `json:"fanOut" jsonschema:"Number of packages this package imports"`
}

// AnalyzeDependenciesOutput contains results from the AnalyzeDependencies tool.
type AnalyzeDependenciesOutput struct {
	// Dependencies - list of packages and their dependencies
	Dependencies []PackageDependency `json:"dependencies" jsonschema:"List of packages and their dependencies"`
	// Cycles - list of dependency cycles found in the project
	Cycles [][]string `json:"cycles" jsonschema:"List of dependency cycles found in the project"`
}

// ------------------ find implementations ------------------.

// FindImplementationsInput contains input data for the FindImplementations tool.
type FindImplementationsInput struct {
	// Dir - root directory of the Go module
	Dir string `json:"dir" jsonschema:"Root directory of the Go module"`
	// Name - name of the interface or type to find implementations for
	Name string `json:"name" jsonschema:"Name of the interface or type to find implementations for"`
}

// Implementation represents an interface implementation.
type Implementation struct {
	// Type - implementing type name
	Type string `json:"type" jsonschema:"Implementing type name"`
	// Interface - interface being implemented
	Interface string `json:"interface" jsonschema:"Interface being implemented"`
	// File - file where the implementation is defined
	File string `json:"file" jsonschema:"File where the implementation is defined"`
	// Line - line number of the implementation
	Line int `json:"line" jsonschema:"Line number of the implementation"`
	// IsType - true if this is a type implementing an interface, false for interface-to-interface embedding
	IsType bool `json:"isType" jsonschema:"True if this is a type implementing an interface, false for interface-to-interface embedding"`
}

// FindImplementationsOutput contains results from the FindImplementations tool.
type FindImplementationsOutput struct {
	// Implementations - list of found implementations
	Implementations []Implementation `json:"implementations" jsonschema:"List of found implementations"`
}

// ------------------ metrics summary ------------------.

// MetricsSummaryInput contains input data for the MetricsSummary tool.
type MetricsSummaryInput struct {
	// Dir - root directory to scan for project metrics
	Dir string `json:"dir" jsonschema:"Root directory to scan for project metrics"`
	// Package - optional package path to restrict metrics aggregation
	Package string `json:"package,omitempty" jsonschema:"Optional Go package path to restrict metrics aggregation"`
}

// MetricsSummaryOutput contains results from the MetricsSummary tool.
type MetricsSummaryOutput struct {
	// PackageCount - total number of packages
	PackageCount int `json:"packageCount" jsonschema:"Total number of packages"`
	// StructCount - total number of structs
	StructCount int `json:"structCount" jsonschema:"Total number of structs"`
	// InterfaceCount - total number of interfaces
	InterfaceCount int `json:"interfaceCount" jsonschema:"Total number of interfaces"`
	// FunctionCount - total number of functions
	FunctionCount int `json:"functionCount" jsonschema:"Total number of functions"`
	// AverageCyclomatic - average cyclomatic complexity across all functions
	AverageCyclomatic float64 `json:"averageCyclomatic" jsonschema:"Average cyclomatic complexity across all functions"`
	// DeadCodeCount - number of unused symbols
	DeadCodeCount int `json:"deadCodeCount" jsonschema:"Number of unused symbols"`
	// ExportedUnusedCount - number of unused exported symbols
	ExportedUnusedCount int `json:"exportedUnusedCount" jsonschema:"Number of exported symbols that are unused"`
	// LineCount - total lines of code
	LineCount int `json:"lineCount" jsonschema:"Total lines of code"`
	// FileCount - total number of Go files
	FileCount int `json:"fileCount" jsonschema:"Total number of Go files"`
}

// ------------------ ast rewrite ------------------.

// ASTRewriteInput contains input data for the ASTRewrite tool.
type ASTRewriteInput struct {
	// Dir - root directory to perform AST rewriting
	Dir string `json:"dir" jsonschema:"Root directory to perform AST rewriting"`
	// Find - pattern to find (e.g., 'pkg.Func(x)')
	Find string `json:"find" jsonschema:"Pattern to find (e.g., 'pkg.Func(x)')"`
	// Replace - pattern to replace with (e.g., 'x.Method()')
	Replace string `json:"replace" jsonschema:"Pattern to replace with (e.g., 'x.Method()')"`
	// DryRun - if true, returns only a preview of changes without writing files
	DryRun bool `json:"dryRun" jsonschema:"If true, only return a diff preview without writing files"`
}

// ASTRewriteOutput contains results from the ASTRewrite tool.
type ASTRewriteOutput struct {
	// ChangedFiles - list of files that were modified
	ChangedFiles []string `json:"changedFiles" jsonschema:"List of files that were modified"`
	// Diffs - diff of changes if dry run was used
	Diffs []FileDiff `json:"diffs,omitempty" jsonschema:"Diff of changes if dry run was used"`
	// TotalChanges - total number of changes made
	TotalChanges int `json:"totalChanges" jsonschema:"Total number of changes made"`
}

// ------------------ read func ------------------

// ReadFuncInput contains input data for the ReadFunc tool.
type ReadFuncInput struct {
	// Dir - root directory of the Go module
	Dir string `json:"dir" jsonschema:"Root directory of the Go module"`
	// Name - function or method name (e.g., 'List' or 'TaskService.List')
	Name string `json:"name" jsonschema:"Function or method name (e.g., 'List' or 'TaskService.List')"`
}

// FunctionSource represents source code of a function or method in Go code.
type FunctionSource struct {
	// Name - function name
	Name string `json:"name" jsonschema:"Function name"`
	// Receiver - receiver type name if this is a method (e.g., 'TaskService')
	Receiver string `json:"receiver,omitempty" jsonschema:"Receiver type name if this is a method (e.g., 'TaskService')"`
	// Package - package path where the function is defined
	Package string `json:"package" jsonschema:"Package path where the function is defined"`
	// File - relative path to the file where the function is defined
	File string `json:"file" jsonschema:"Relative path to the file where the function is defined"`
	// StartLine - starting line of the function
	StartLine int `json:"startLine" jsonschema:"Starting line number of the function"`
	// EndLine - ending line of the function
	EndLine int `json:"endLine" jsonschema:"Ending line number of the function"`
	// SourceCode - full source code of the function
	SourceCode string `json:"sourceCode" jsonschema:"Full source code of the function or method"`
}

// ReadFuncOutput contains results from the ReadFunc tool.
type ReadFuncOutput struct {
	// Function - found function with metadata and source code
	Function FunctionSource `json:"function" jsonschema:"Extracted function with metadata and source code"`
}

// ------------------ read file ------------------

// ReadFileInput contains input data for the ReadFile tool.
type ReadFileInput struct {
	// Dir - root directory of the project (Go module)
	Dir string `json:"dir" jsonschema:"Root directory of the Go module"`
	// File - relative path to the file to read
	File string `json:"file" jsonschema:"Relative path to the Go source file to read"`
	// Mode - read mode: "raw" (text only), "summary" (package, imports, symbols, lines), "ast" (full AST analysis)
	Mode string `json:"mode,omitempty" jsonschema:"Read mode: raw, summary, or ast"`
}

// ReadFileOutput contains results from the ReadFile tool.
type ReadFileOutput struct {
	// File - path to the read file
	File string `json:"file" jsonschema:"File path that was read"`
	// Package - name of the package declared in the file
	Package string `json:"package,omitempty" jsonschema:"Declared Go package name"`
	// Imports - list of imported packages
	Imports []Import `json:"imports,omitempty" jsonschema:"List of imported packages in the file"`
	// Symbols - functions, structs, interfaces, constants, etc.
	Symbols []Symbol `json:"symbols,omitempty" jsonschema:"List of declared symbols within the file"`
	// LineCount - total number of lines in the file
	LineCount int `json:"lineCount" jsonschema:"Total number of lines in the file"`
	// Source - source code of the file (if requested mode is raw or ast)
	Source string `json:"source,omitempty" jsonschema:"Full source code of the file if requested"`
}

// ------------------ read struct ------------------

// ReadStructInput contains input data for the ReadStruct tool.
type ReadStructInput struct {
	// Dir - root directory of the Go module
	Dir string `json:"dir" jsonschema:"Root directory of the Go module"`
	// Name - name of the struct to read (e.g., 'User' or 'models.User')
	Name string `json:"name" jsonschema:"Name of the struct to read (e.g., 'User' or 'models.User')"`
	// IncludeMethods - if true, also returns methods of the struct
	IncludeMethods bool `json:"includeMethods,omitempty" jsonschema:"If true, also include methods of the struct"`
}

// StructField represents a single field of a struct.
type StructField struct {
	// Name - field name
	Name string `json:"name" jsonschema:"Field name"`
	// Type - field type (e.g., string, int, time.Time)
	Type string `json:"type" jsonschema:"Field type"`
	// Tag - struct tag value (e.g., json:"id,omitempty")
	Tag string `json:"tag,omitempty" jsonschema:"Struct tag value"`
	// Doc - field comment if any
	Doc string `json:"doc,omitempty" jsonschema:"Field documentation comment"`
}

// StructInfo represents struct declaration.
type StructInfo struct {
	// Name - struct name
	Name string `json:"name" jsonschema:"Struct name"`
	// Package - package name where the struct is defined
	Package string `json:"package" jsonschema:"Package where the struct is defined"`
	// File - relative path to the file where the struct is defined
	File string `json:"file" jsonschema:"File where the struct is defined"`
	// Line - line number of struct declaration
	Line int `json:"line" jsonschema:"Line number where the struct is declared"`
	// Exported - true if the struct is exported
	Exported bool `json:"exported" jsonschema:"True if the struct is exported"`
	// Doc - documentation above the struct (comment)
	Doc string `json:"doc,omitempty" jsonschema:"Struct documentation comment"`
	// Fields - list of struct fields
	Fields []StructField `json:"fields" jsonschema:"List of struct fields"`
	// Methods - list of struct methods, if IncludeMethods = true
	Methods []string `json:"methods,omitempty" jsonschema:"List of methods belonging to the struct"`
	// Source - source code of struct declaration
	Source string `json:"source" jsonschema:"Full struct source code"`
}

// ReadStructOutput contains results from the ReadStruct tool.
type ReadStructOutput struct {
	// Struct - description of the found struct
	Struct StructInfo `json:"struct" jsonschema:"Description of the found struct"`
}

// ------------------ project schema ------------------

// ProjectSchemaInput defines input parameters for the ProjectSchema tool.
type ProjectSchemaInput struct {
	// Dir - root directory of the Go module to analyze
	Dir string `json:"dir" jsonschema:"Root directory of the Go module to analyze"`

	// Depth - level of analysis detail: "summary", "standard", or "deep"
	Depth string `json:"depth,omitempty" jsonschema:"Level of analysis detail: summary, standard, or deep"`
}

// ProjectPackageSymbols represents exported symbols within a package.
type ProjectPackageSymbols struct {
	// Structs - list of struct type names
	Structs []string `json:"structs,omitempty" jsonschema:"List of struct type names"`
	// Interfaces - list of interface type names
	Interfaces []string `json:"interfaces,omitempty" jsonschema:"List of interface type names"`
	// Functions - list of function names
	Functions []string `json:"functions,omitempty" jsonschema:"List of function names"`
	// Types - list of additional named types
	Types []string `json:"types,omitempty" jsonschema:"List of additional named types"`
}

// ProjectPackage describes a Go package and its relationships.
type ProjectPackage struct {
	// Path - full import path of the package
	Path string `json:"path" jsonschema:"Full import path of the package"`
	// Name - short package name
	Name string `json:"name" jsonschema:"Short package name"`
	// Imports - list of imported package paths
	Imports []string `json:"imports,omitempty" jsonschema:"List of imported package paths"`
	// Symbols - exported symbols defined in the package
	Symbols ProjectPackageSymbols `json:"symbols,omitempty" jsonschema:"Exported symbols defined in the package"`
}

// ProjectInterface represents an interface definition across the module.
type ProjectInterface struct {
	// Name - interface name
	Name string `json:"name" jsonschema:"Interface name"`
	// Methods - list of method names defined in the interface
	Methods []string `json:"methods,omitempty" jsonschema:"List of method names defined in the interface"`
	// DefinedIn - package path where the interface is defined
	DefinedIn string `json:"definedIn" jsonschema:"Package path where the interface is defined"`
}

// ProjectDependencyGraph represents inter-package dependencies.
type ProjectDependencyGraph map[string][]string

// ProjectSummary provides aggregated statistics about the project.
type ProjectSummary struct {
	// PackageCount - total number of packages analyzed
	PackageCount int `json:"packageCount" jsonschema:"Total number of packages analyzed"`
	// FunctionCount - total number of functions found
	FunctionCount int `json:"functionCount" jsonschema:"Total number of functions found"`
	// StructCount - total number of struct types found
	StructCount int `json:"structCount" jsonschema:"Total number of struct types found"`
	// InterfaceCount - total number of interfaces found
	InterfaceCount int `json:"interfaceCount" jsonschema:"Total number of interfaces found"`
}

// ProjectSchemaOutput contains a structured representation of a Go project's architecture.
type ProjectSchemaOutput struct {
	// Module - Go module name from go.mod
	Module string `json:"module" jsonschema:"Go module name from go.mod"`
	// GoVersion - Go language version declared in go.mod
	GoVersion string `json:"goVersion,omitempty" jsonschema:"Go language version declared in go.mod"`
	// RootDir - absolute path to the analyzed module root
	RootDir string `json:"rootDir" jsonschema:"Absolute path to the analyzed module root"`
	// Packages - list of analyzed packages with symbols and imports
	Packages []ProjectPackage `json:"packages,omitempty" jsonschema:"List of analyzed packages with symbols and imports"`
	// Interfaces - list of all interfaces defined across the project
	Interfaces []ProjectInterface `json:"interfaces,omitempty" jsonschema:"List of all interfaces defined across the project"`
	// ExternalDeps - list of external module dependencies (excluding stdlib and internal packages)
	ExternalDeps []string `json:"externalDeps,omitempty" jsonschema:"List of external module dependencies"`
	// DependencyGraph - package-to-package import graph
	DependencyGraph ProjectDependencyGraph `json:"dependencyGraph,omitempty" jsonschema:"Package-to-package import graph"`
	// Summary - aggregated counts of key code entities
	Summary ProjectSummary `json:"summary,omitempty" jsonschema:"Aggregated counts of key code entities"`
}
