package tools

type ListPackagesInput struct {
	Dir string `json:"dir" jsonschema:"directory to scan for packages"`
}

type ListPackagesOutput struct {
	Packages []string `json:"packages" jsonschema:"list of package paths"`
}

type ListSymbolsInput struct {
	Dir     string `json:"dir"     jsonschema:"directory to scan for packages"`
	Package string `json:"package" jsonschema:"package path to inspect"`
}

type Symbol struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	File string `json:"file"`
	Line int    `json:"line"`
}

type ListSymbolsOutput struct {
	Symbols []Symbol `json:"symbols"`
}

type FindReferencesInput struct {
	Dir   string `json:"dir"   jsonschema:"directory to scan"`
	Ident string `json:"ident" jsonschema:"identifier to search for"`
}

type Reference struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Snippet string `json:"snippet"`
}

type FindReferencesOutput struct {
	References []Reference `json:"references"`
}

type FindDefinitionsInput struct {
	Dir   string `json:"dir"   jsonschema:"directory to scan"`
	Ident string `json:"ident" jsonschema:"identifier to search for definition"`
}

type Definition struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Snippet string `json:"snippet"`
}

type FindDefinitionsOutput struct {
	Definitions []Definition `json:"definitions"`
}

type RenameSymbolInput struct {
	Dir     string `json:"dir"     jsonschema:"directory to scan"`
	OldName string `json:"oldName" jsonschema:"symbol name to rename"`
	NewName string `json:"newName" jsonschema:"new symbol name"`
}

type RenameSymbolOutput struct {
	ChangedFiles []string `json:"changedFiles"`
}

type ListImportsInput struct {
	Dir string `json:"dir" jsonschema:"directory to scan for Go files"`
}

type Import struct {
	Path string `json:"path"`
	File string `json:"file"`
	Line int    `json:"line"`
}

type ListImportsOutput struct {
	Imports []Import `json:"imports"`
}

type ListInterfacesInput struct {
	Dir string `json:"dir" jsonschema:"directory to scan for Go files"`
}

type InterfaceMethod struct {
	Name string `json:"name"`
	Line int    `json:"line"`
}

type InterfaceInfo struct {
	Name    string            `json:"name"`
	File    string            `json:"file"`
	Line    int               `json:"line"`
	Methods []InterfaceMethod `json:"methods"`
}

type ListInterfacesOutput struct {
	Interfaces []InterfaceInfo `json:"interfaces"`
}

type AnalyzeComplexityInput struct {
	Dir string `json:"dir" jsonschema:"directory to scan for Go files"`
}

type FunctionComplexity struct {
	Name       string `json:"name"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	Lines      int    `json:"lines"`
	Nesting    int    `json:"nesting"`
	Cyclomatic int    `json:"cyclomatic"`
}

type AnalyzeComplexityOutput struct {
	Functions []FunctionComplexity `json:"functions"`
}

type DeadCodeInput struct {
	Dir string `json:"dir" jsonschema:"directory to scan for Go files"`
}

type DeadSymbol struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	File string `json:"file"`
	Line int    `json:"line"`
}

type DeadCodeOutput struct {
	Unused []DeadSymbol `json:"unused"`
}
