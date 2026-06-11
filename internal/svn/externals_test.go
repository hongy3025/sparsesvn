package svn

import (
	"context"
	"testing"
)

func TestParseExternalsOutputOldFormat(t *testing.T) {
	// Old format (SVN 1.4): target [-rN] URL
	output := "utils svn://server/repo/trunk/utils\nproto -r42 svn://server/repo/trunk/proto\n"
	extDefs, err := ParseExternalsOutput(output)
	if err != nil {
		t.Fatalf("ParseExternalsOutput: %v", err)
	}
	if len(extDefs) != 2 {
		t.Fatalf("expected 2 externals, got %d", len(extDefs))
	}
	if extDefs["utils"].URL != "svn://server/repo/trunk/utils" {
		t.Errorf("utils URL = %q", extDefs["utils"].URL)
	}
	if extDefs["utils"].Revision != "" {
		t.Errorf("utils Revision = %q, want empty", extDefs["utils"].Revision)
	}
	if extDefs["proto"].URL != "svn://server/repo/trunk/proto" {
		t.Errorf("proto URL = %q", extDefs["proto"].URL)
	}
	if extDefs["proto"].Revision != "42" {
		t.Errorf("proto Revision = %q, want 42", extDefs["proto"].Revision)
	}
}

func TestParseExternalsOutputNewFormat(t *testing.T) {
	// New format (SVN 1.5+): [-rN] URL target
	output := "svn://server/repo/trunk/utils utils\n-r42 svn://server/repo/trunk/proto proto\n"
	extDefs, err := ParseExternalsOutput(output)
	if err != nil {
		t.Fatalf("ParseExternalsOutput: %v", err)
	}
	if extDefs["utils"].URL != "svn://server/repo/trunk/utils" {
		t.Errorf("utils URL = %q", extDefs["utils"].URL)
	}
	if extDefs["proto"].Revision != "42" {
		t.Errorf("proto Revision = %q, want 42", extDefs["proto"].Revision)
	}
}

func TestCheckoutExternalArgs(t *testing.T) {
	fc := &FakeClient{}
	ctx := context.Background()
	err := CheckoutExternal(ctx, fc, "/workdir", "src/core", "lib", "svn://server/repo/trunk/utils", "files", "", "")
	if err != nil {
		t.Fatalf("CheckoutExternal: %v", err)
	}
	if len(fc.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(fc.Calls))
	}
	args := fc.Calls[0].Args
	foundIgnore := false
	foundDepth := false
	for _, arg := range args {
		if arg == "--ignore-externals" {
			foundIgnore = true
		}
		if arg == "files" {
			foundDepth = true
		}
	}
	if !foundIgnore {
		t.Error("expected --ignore-externals in args")
	}
	if !foundDepth {
		t.Error("expected 'files' depth in args")
	}
}

func TestCheckoutIgnoresExternals(t *testing.T) {
	fc := &FakeClient{}
	ctx := context.Background()
	err := Checkout(ctx, fc, "/workdir", "svn://server/repo/trunk", "")
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	args := fc.Calls[0].Args
	found := false
	for _, arg := range args {
		if arg == "--ignore-externals" {
			found = true
		}
	}
	if !found {
		t.Error("Checkout should include --ignore-externals")
	}
}

func TestSetDepthIgnoresExternals(t *testing.T) {
	fc := &FakeClient{}
	ctx := context.Background()
	err := SetDepth(ctx, fc, "/workdir", "src", 2, "") // 2 = DepthInfinity
	if err != nil {
		t.Fatalf("SetDepth: %v", err)
	}
	args := fc.Calls[0].Args
	found := false
	for _, arg := range args {
		if arg == "--ignore-externals" {
			found = true
		}
	}
	if !found {
		t.Error("SetDepth should include --ignore-externals")
	}
}

func TestFakeClientStdoutByArgs(t *testing.T) {
	fc := &FakeClient{
		StdoutByArgs: []StdoutMatch{
			{ArgsContains: []string{"propget"}, Stdout: "utils svn://server/repo/trunk/utils\n"},
		},
	}
	ctx := context.Background()
	result, err := fc.Run(ctx, "/workdir", "propget", "svn:externals", "src")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Stdout != "utils svn://server/repo/trunk/utils\n" {
		t.Errorf("Stdout = %q, want externals output", result.Stdout)
	}
	// Non-matching call should return empty
	result2, _ := fc.Run(ctx, "/workdir", "update", "src")
	if result2.Stdout != "" {
		t.Errorf("Non-matching Stdout = %q, want empty", result2.Stdout)
	}
}
