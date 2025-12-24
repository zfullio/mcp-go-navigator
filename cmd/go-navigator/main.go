package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go-navigator/internal/tools"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "go-navigator",
			Title:   "Go Navigator",
			Version: "v1.5.0",
		},
		&mcp.ServerOptions{
			Instructions: strings.TrimSpace(`
🧭 You are Go Navigator — a semantic Go code analysis and refactoring assistant.

Capabilities
- Explore Go project structure and dependencies
- Analyze symbols, interfaces, and function complexity
- Detect dead code and unused symbols
- Perform safe, type-aware renames with dry-run diff output

Usage
- Run tools from the Go module root (directory containing go.mod)
- Pass "dir" to specify the analysis root
- Prefer semantic analysis tools over text search for accuracy
            `),
		},
	)

	mcp.AddTool[tools.ListPackagesInput, tools.ListPackagesOutput](server, &mcp.Tool{
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint: true,
		},
		Description: tools.ListPackagesDesc,
		Name:        "listPackages",
		Title:       "List Packages",
	}, tools.ListPackages)

	mcp.AddTool[tools.ListSymbolsInput, tools.ListSymbolsOutput](server, &mcp.Tool{
		Name:  "listSymbols",
		Title: "List Symbols",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint: true,
		},
		Description: tools.ListSymbolsDesc,
	}, tools.ListSymbols)

	mcp.AddTool[tools.FindDefinitionsInput, tools.FindDefinitionsOutput](server, &mcp.Tool{
		Name:  "getDefinitions",
		Title: "Get Definitions",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint: true,
		},
		Description: tools.GetDefinitionsDesc,
	}, tools.FindDefinitions)

	mcp.AddTool[tools.FindReferencesInput, tools.FindReferencesOutput](server, &mcp.Tool{
		Name:  "getReferences",
		Title: "Get References",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint: true,
		},
		Description: tools.GetReferencesDesc,
	}, tools.FindReferences)

	mcp.AddTool[tools.FindBestContextInput, tools.FindBestContextOutput](server, &mcp.Tool{
		Name:  "getSymbolContext",
		Title: "Get Symbol Context",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint: true,
		},
		Description: tools.GetSymbolContextDesc,
	}, tools.FindBestContext)

	mcp.AddTool[tools.RenameSymbolInput, tools.RenameSymbolOutput](server, &mcp.Tool{
		Name:  "renameSymbol",
		Title: "Rename Symbol",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint: false,
		},
		Description: tools.RenameSymbolDesc,
	}, tools.RenameSymbol)

	mcp.AddTool[tools.ListImportsInput, tools.ListImportsOutput](server, &mcp.Tool{
		Name:  "listImports",
		Title: "List Imports",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint: true,
		},
		Description: tools.ListImportsDesc,
	}, tools.ListImports)

	mcp.AddTool[tools.ListInterfacesInput, tools.ListInterfacesOutput](server, &mcp.Tool{
		Name:  "listInterfaces",
		Title: "List Interfaces",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint: true,
		},
		Description: tools.ListInterfacesDesc,
	}, tools.ListInterfaces)

	mcp.AddTool[tools.AnalyzeComplexityInput, tools.AnalyzeComplexityOutput](server, &mcp.Tool{
		Name:  "getComplexityReport",
		Title: "Get Complexity Report",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint: true,
		},
		Description: tools.GetComplexityReportDesc,
	}, tools.AnalyzeComplexity)

	mcp.AddTool[tools.DeadCodeInput, tools.DeadCodeOutput](server, &mcp.Tool{
		Name:  "getDeadCodeReport",
		Title: "Get Dead Code Report",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint: true,
		},
		Description: tools.GetDeadCodeReportDesc,
	}, tools.DeadCode)

	mcp.AddTool[tools.AnalyzeDependenciesInput, tools.AnalyzeDependenciesOutput](server, &mcp.Tool{
		Name:  "getDependencyGraph",
		Title: "Get Dependency Graph",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint: true,
		},
		Description: tools.GetDependencyGraphDesc,
	}, tools.AnalyzeDependencies)

	mcp.AddTool[tools.FindImplementationsInput, tools.FindImplementationsOutput](server, &mcp.Tool{
		Name:  "getImplementations",
		Title: "Get Implementations",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint: true,
		},
		Description: tools.GetImplementationsDesc,
	}, tools.FindImplementations)

	mcp.AddTool[tools.ASTRewriteInput, tools.ASTRewriteOutput](server, &mcp.Tool{
		Name:  "rewriteAst",
		Title: "Rewrite AST (Semantic)",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint: false,
		},
		Description: tools.RewriteAstDesc,
	}, tools.ASTRewrite)

	mcp.AddTool[tools.ReadFuncInput, tools.ReadFuncOutput](server, &mcp.Tool{
		Name:  "getFunctionSource",
		Title: "Get Function Source",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint: true,
		},
		Description: tools.GetFunctionSourceDesc,
	}, tools.ReadFunc)

	mcp.AddTool[tools.ReadGoFileInput, tools.ReadGoFileOutput](server, &mcp.Tool{
		Name:  "getFileInfo",
		Title: "Get File Info",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint: true,
		},
		Description: tools.GetFileInfoDesc,
	}, tools.ReadGoFile)

	mcp.AddTool[tools.ReadStructInput, tools.ReadStructOutput](server, &mcp.Tool{
		Name:  "getStructInfo",
		Title: "Get Struct Info",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint: true,
		},
		Description: tools.GetStructInfoDesc,
	}, tools.ReadStruct)

	mcp.AddTool[tools.ProjectSchemaInput, tools.ProjectSchemaOutput](server, &mcp.Tool{
		Name:  "getProjectSchema",
		Title: "Get Project Schema",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
		Description: tools.GetProjectSchemaDesc,
	}, tools.ProjectSchema)

	err := tools.HealthCheck()
	if err != nil {
		log.Warn().Err(err).Msg("initial health check failed (non-fatal)")
	} else {
		log.Info().Msg("health check passed")
	}

	log.Info().Msg("🚀 go-navigator MCP server started (press Ctrl+C to stop)")

	go func() {
		err := server.Run(ctx, &mcp.StdioTransport{})
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Fatal().Err(err).Msg("server terminated with error")
		} else {
			log.Info().Msg("server stopped cleanly")
		}
	}()

	<-ctx.Done()
	log.Info().Msg("🛑 go-navigator MCP server stopped gracefully")

	time.Sleep(200 * time.Millisecond)
	os.Stderr.Sync()
}
