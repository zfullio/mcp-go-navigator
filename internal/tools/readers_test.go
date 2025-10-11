package tools_test

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go-navigator/internal/tools"
)

func TestReadGoFile_Summary(t *testing.T) {
	t.Parallel()

	in := tools.ReadGoFileInput{
		Dir:  testDir(),
		File: "foo.go",
		Options: tools.ReadGoFileOptions{
			WithSource:   false,
			WithComments: false,
		},
	}

	_, out, err := tools.ReadGoFile(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("ReadGoFile error: %v", err)
	}

	if out.Package != "sample" {
		t.Errorf("expected package sample, got %s", out.Package)
	}

	if out.Source != "" {
		t.Errorf("expected source to be empty when WithSource=false")
	}

	if len(out.Symbols) == 0 {
		t.Fatalf("expected symbols to be present, got none")
	}

	seen := map[string]string{}
	for _, sym := range out.Symbols {
		seen[sym.Name] = sym.Kind
	}

	if seen["Foo"] != "struct" {
		t.Errorf("expected Foo struct in symbols, got kind %s", seen["Foo"])
	}

	if seen["DoSomething"] != "func" {
		t.Errorf("expected DoSomething function in symbols, got kind %s", seen["DoSomething"])
	}

	if seen["unusedConst"] != "const" {
		t.Errorf("expected unusedConst const in symbols, got kind %s", seen["unusedConst"])
	}
}

func TestReadGoFile_WithSource(t *testing.T) {
	t.Parallel()

	in := tools.ReadGoFileInput{
		Dir:  testDir(),
		File: "foo.go",
		Options: tools.ReadGoFileOptions{
			WithSource: true,
		},
	}

	_, out, err := tools.ReadGoFile(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("ReadGoFile error: %v", err)
	}

	if out.Source == "" {
		t.Fatalf("expected source content when WithSource=true")
	}

	if out.Package == "" {
		t.Errorf("expected package name, got empty string")
	}

	if len(out.Imports) == 0 {
		t.Errorf("expected imports to be parsed, got none")
	}
}

func TestReadFunc_Method(t *testing.T) {
	t.Parallel()

	in := tools.ReadFuncInput{Dir: testDir(), Name: "Foo.DoSomething"}

	_, out, err := tools.ReadFunc(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("ReadFunc error: %v", err)
	}

	fn := out.Function
	if fn.Name != "DoSomething" {
		t.Fatalf("expected function name DoSomething, got %s", fn.Name)
	}

	if fn.Receiver != "Foo" {
		t.Errorf("expected receiver Foo, got %s", fn.Receiver)
	}

	if fn.File != "foo.go" {
		t.Errorf("expected file foo.go, got %s", fn.File)
	}

	if !strings.Contains(fn.SourceCode, "return strings.ToUpper") {
		t.Errorf("expected source code to contain body, got %q", fn.SourceCode)
	}
}

func TestReadStruct_WithMethods(t *testing.T) {
	t.Parallel()

	in := tools.ReadStructInput{Dir: testDir(), Name: "Foo", IncludeMethods: true}

	_, out, err := tools.ReadStruct(context.Background(), &mcp.CallToolRequest{}, in)
	if err != nil {
		t.Fatalf("ReadStruct error: %v", err)
	}

	st := out.Struct
	if st.Name != "Foo" {
		t.Fatalf("expected struct Foo, got %s", st.Name)
	}

	if len(st.Fields) == 0 {
		t.Fatalf("expected struct fields, got 0")
	}

	foundID := false

	for _, field := range st.Fields {
		if field.Name == "ID" && field.Type == "int" {
			foundID = true
		}
	}

	if !foundID {
		t.Errorf("expected field ID int in struct fields")
	}

	if !containsAll(st.Methods, "DoSomething", "deadHelper") {
		t.Errorf("expected methods DoSomething and deadHelper, got %v", st.Methods)
	}
}
