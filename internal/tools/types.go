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
	Kind string `json:"kind" jsonschema:"Symbol type (func, struct, interface, method, etc.)"`
	Name string `json:"name" jsonschema:"Symbol name"`
	File string `json:"file" jsonschema:"File where the symbol is defined"`
	Line int    `json:"line" jsonschema:"Line number in the file"`
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
	Dir string `json:"dir" jsonschema:"Root directory to scan for unused symbols"`
}

type DeadSymbol struct {
	Name string `json:"name" jsonschema:"Symbol name"`
	Kind string `json:"kind" jsonschema:"Symbol kind (func, var, const, type)"`
	File string `json:"file" jsonschema:"File where the unused symbol is declared"`
	Line int    `json:"line" jsonschema:"Line number of the symbol"`
}

type DeadCodeOutput struct {
	Unused []DeadSymbol `json:"unused" jsonschema:"List of unused or dead code symbols"`
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
