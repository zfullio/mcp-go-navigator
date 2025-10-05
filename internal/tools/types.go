package tools

// ------------------ list packages ------------------

type ListPackagesInput struct {
	Dir string `json:"dir" jsonschema:"Root directory to scan for Go packages"`
}

type ListPackagesOutput struct {
	Packages []string `json:"packages" jsonschema:"List of discovered Go package import paths"`
}

// ------------------ list symbols ------------------

type ListSymbolsInput struct {
	Dir     string `json:"dir"     jsonschema:"Root directory of the Go module"`
	Package string `json:"package" jsonschema:"Package path to inspect for symbols"`
}

type Symbol struct {
	Kind     string `json:"kind"     jsonschema:"Symbol type (func, struct, interface, method, etc.)"`
	Name     string `json:"name"     jsonschema:"Symbol name"`
	Package  string `json:"package"  jsonschema:"Package where the symbol is defined"`
	File     string `json:"file"     jsonschema:"File where the symbol is defined"`
	Line     int    `json:"line"     jsonschema:"Line number in the file"`
	Exported bool   `json:"exported" jsonschema:"True if the symbol is exported (starts with capital letter)"`
}

type ListSymbolsOutput struct {
	Symbols []Symbol `json:"symbols" jsonschema:"All discovered symbols within the specified package"`
}

// ------------------ find references ------------------

type FindReferencesInput struct {
	Dir   string `json:"dir"            jsonschema:"Root directory of the Go module"`
	Ident string `json:"ident"          jsonschema:"Name of the symbol to find references for"`
	File  string `json:"file,omitempty" jsonschema:"Optional relative file path to restrict the search"`
	Kind  string `json:"kind,omitempty" jsonschema:"Filter by symbol kind (e.g. func, type, var, const)"`
}

type Reference struct {
	File    string `json:"file"    jsonschema:"Relative path to the file containing the reference"`
	Line    int    `json:"line"    jsonschema:"Line number of the reference"`
	Snippet string `json:"snippet" jsonschema:"Code context showing the reference usage"`
}

type FindReferencesOutput struct {
	References []Reference `json:"references" jsonschema:"List of all found references to the given identifier"`
}

// ------------------ find definitions ------------------

type FindDefinitionsInput struct {
	Dir   string `json:"dir"            jsonschema:"Root directory of the Go module"`
	Ident string `json:"ident"          jsonschema:"Name of the symbol to locate its definition"`
	File  string `json:"file,omitempty" jsonschema:"Optional relative file path to restrict the search"`
	Kind  string `json:"kind,omitempty" jsonschema:"Filter by symbol kind (e.g. func, type, var, const)"`
}

type Definition struct {
	File    string `json:"file"    jsonschema:"Relative path to the file where the symbol is defined"`
	Line    int    `json:"line"    jsonschema:"Line number of the definition"`
	Snippet string `json:"snippet" jsonschema:"Code snippet showing the definition line"`
}

type FindDefinitionsOutput struct {
	Definitions []Definition `json:"definitions" jsonschema:"List of found symbol definitions"`
}

// ------------------ list imports ------------------

type ListImportsInput struct {
	Dir string `json:"dir" jsonschema:"Root directory to scan for Go files"`
}

type Import struct {
	Path string `json:"path" jsonschema:"Imported package path"`
	File string `json:"file" jsonschema:"File where the import is declared"`
	Line int    `json:"line" jsonschema:"Line number of the import statement"`
}

type ListImportsOutput struct {
	Imports []Import `json:"imports" jsonschema:"All imports found in scanned Go files"`
}

// ------------------ list interfaces ------------------

type ListInterfacesInput struct {
	Dir string `json:"dir" jsonschema:"Root directory to scan for Go files"`
}

type InterfaceMethod struct {
	Name string `json:"name" jsonschema:"Method name"`
	Line int    `json:"line" jsonschema:"Line number of the method"`
}

type InterfaceInfo struct {
	Name    string            `json:"name"    jsonschema:"Interface name"`
	File    string            `json:"file"    jsonschema:"File where the interface is defined"`
	Line    int               `json:"line"    jsonschema:"Line number of the interface declaration"`
	Methods []InterfaceMethod `json:"methods" jsonschema:"List of methods defined in the interface"`
}

type ListInterfacesOutput struct {
	Interfaces []InterfaceInfo `json:"interfaces" jsonschema:"All interfaces found in scanned Go files"`
}

// ------------------ analyze complexity ------------------

type AnalyzeComplexityInput struct {
	Dir string `json:"dir" jsonschema:"Root directory to scan for Go files"`
}

type FunctionComplexity struct {
	Name       string `json:"name"       jsonschema:"Function name"`
	File       string `json:"file"       jsonschema:"File where the function is defined"`
	Line       int    `json:"line"       jsonschema:"Line number of the function"`
	Lines      int    `json:"lines"      jsonschema:"Total number of lines in the function"`
	Nesting    int    `json:"nesting"    jsonschema:"Maximum nesting depth"`
	Cyclomatic int    `json:"cyclomatic" jsonschema:"Cyclomatic complexity value"`
}

type AnalyzeComplexityOutput struct {
	Functions []FunctionComplexity `json:"functions" jsonschema:"Calculated complexity metrics for all functions"`
}

// ------------------ dead code ------------------

type DeadCodeInput struct {
	Dir             string `json:"dir"                       jsonschema:"Root directory to scan for unused symbols"`
	IncludeExported bool   `json:"includeExported,omitempty" jsonschema:"If true, include exported symbols that are unused"`
}

type DeadSymbol struct {
	Name       string `json:"name"       jsonschema:"Symbol name"`
	Kind       string `json:"kind"       jsonschema:"Symbol kind (func, var, const, type)"`
	File       string `json:"file"       jsonschema:"File where the unused symbol is declared"`
	Line       int    `json:"line"       jsonschema:"Line number of the symbol"`
	IsExported bool   `json:"isExported" jsonschema:"True if the symbol is exported (starts with capital letter)"`
	Package    string `json:"package"    jsonschema:"Package where the symbol is defined"`
}

type DeadCodeOutput struct {
	Unused        []DeadSymbol   `json:"unused"        jsonschema:"List of unused or dead code symbols"`
	TotalCount    int            `json:"totalCount"    jsonschema:"Total number of unused symbols found"`
	ExportedCount int            `json:"exportedCount" jsonschema:"Number of exported symbols that are unused"`
	ByPackage     map[string]int `json:"byPackage"     jsonschema:"Count of unused symbols grouped by package"`
}

// ------------------ rename symbol ------------------

type RenameSymbolInput struct {
	Dir     string `json:"dir"              jsonschema:"Root directory of the Go module"`
	OldName string `json:"oldName"          jsonschema:"Current symbol name to rename"`
	NewName string `json:"newName"          jsonschema:"New symbol name to apply"`
	Kind    string `json:"kind,omitempty"   jsonschema:"Symbol kind: func, var, const, type, package"`
	DryRun  bool   `json:"dryRun,omitempty" jsonschema:"If true, only return a diff preview without writing files"`
}

type FileDiff struct {
	Path string `json:"path" jsonschema:"File path where changes occurred"`
	Diff string `json:"diff" jsonschema:"Unified diff showing changes"`
}

type RenameSymbolOutput struct {
	ChangedFiles []string   `json:"changedFiles"         jsonschema:"List of modified files"`
	Diffs        []FileDiff `json:"diffs,omitempty"      jsonschema:"Diff results if dry run was used"`
	Collisions   []string   `json:"collisions,omitempty" jsonschema:"List of name conflicts preventing rename"`
}

// ------------------ analyze dependencies ------------------.
type AnalyzeDependenciesInput struct {
	Dir string `json:"dir" jsonschema:"Root directory to scan for package dependencies"`
}

type PackageDependency struct {
	Package string   `json:"package" jsonschema:"Package path"`
	Imports []string `json:"imports" jsonschema:"List of imported packages"`
	FanIn   int      `json:"fanIn"   jsonschema:"Number of other packages that import this package"`
	FanOut  int      `json:"fanOut"  jsonschema:"Number of packages this package imports"`
}

type AnalyzeDependenciesOutput struct {
	Dependencies []PackageDependency `json:"dependencies" jsonschema:"List of packages and their dependencies"`
	Cycles       [][]string          `json:"cycles"       jsonschema:"List of dependency cycles found in the project"`
}

// ------------------ find implementations ------------------.
type FindImplementationsInput struct {
	Dir  string `json:"dir"  jsonschema:"Root directory of the Go module"`
	Name string `json:"name" jsonschema:"Name of the interface or type to find implementations for"`
}

type Implementation struct {
	Type      string `json:"type"      jsonschema:"Implementing type name"`
	Interface string `json:"interface" jsonschema:"Interface being implemented"`
	File      string `json:"file"      jsonschema:"File where the implementation is defined"`
	Line      int    `json:"line"      jsonschema:"Line number of the implementation"`
	IsType    bool   `json:"isType"    jsonschema:"True if this is a type implementing an interface, false for interface-to-interface embedding"`
}

type FindImplementationsOutput struct {
	Implementations []Implementation `json:"implementations" jsonschema:"List of found implementations"`
}

// ------------------ metrics summary ------------------.
type MetricsSummaryInput struct {
	Dir string `json:"dir" jsonschema:"Root directory to scan for project metrics"`
}

type MetricsSummaryOutput struct {
	PackageCount        int     `json:"packageCount"        jsonschema:"Total number of packages"`
	StructCount         int     `json:"structCount"         jsonschema:"Total number of structs"`
	InterfaceCount      int     `json:"interfaceCount"      jsonschema:"Total number of interfaces"`
	FunctionCount       int     `json:"functionCount"       jsonschema:"Total number of functions"`
	AverageCyclomatic   float64 `json:"averageCyclomatic"   jsonschema:"Average cyclomatic complexity across all functions"`
	DeadCodeCount       int     `json:"deadCodeCount"       jsonschema:"Number of unused symbols"`
	ExportedUnusedCount int     `json:"exportedUnusedCount" jsonschema:"Number of exported symbols that are unused"`
	LineCount           int     `json:"lineCount"           jsonschema:"Total lines of code"`
	FileCount           int     `json:"fileCount"           jsonschema:"Total number of Go files"`
}

// ------------------ ast rewrite ------------------.
type ASTRewriteInput struct {
	Dir     string `json:"dir"     jsonschema:"Root directory to perform AST rewriting"`
	Find    string `json:"find"    jsonschema:"Pattern to find (e.g., 'pkg.Func(x)')"`
	Replace string `json:"replace" jsonschema:"Pattern to replace with (e.g., 'x.Method()')"`
	DryRun  bool   `json:"dryRun"  jsonschema:"If true, only return a diff preview without writing files"`
}

type ASTRewriteOutput struct {
	ChangedFiles []string   `json:"changedFiles"    jsonschema:"List of files that were modified"`
	Diffs        []FileDiff `json:"diffs,omitempty" jsonschema:"Diff of changes if dry run was used"`
	TotalChanges int        `json:"totalChanges"    jsonschema:"Total number of changes made"`
}
