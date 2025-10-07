package tools

import "golang.org/x/tools/go/packages"

const (
	loadModeBasic                 packages.LoadMode = packages.NeedName | packages.NeedCompiledGoFiles
	loadModeSyntaxTypes                             = packages.NeedSyntax | packages.NeedTypes | packages.NeedCompiledGoFiles | packages.NeedTypesInfo
	loadModeSyntaxTypesNamed                        = loadModeSyntaxTypes | packages.NeedName
	loadModeBasicSyntax                             = loadModeBasic | packages.NeedSyntax
	loadModeSyntaxTypesNamedFiles                   = loadModeSyntaxTypesNamed | packages.NeedFiles
)
