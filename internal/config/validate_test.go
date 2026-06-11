package config

import (
	"strings"
	"testing"
)

func TestValidateExternalsTargetEmpty(t *testing.T) {
	cfg := &Config{
		Paths: []PathSpec{
			{Path: "src", Depth: DepthInfinity, Externals: []ExternalSpec{
				{Target: "", Depth: DepthFiles},
			}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for empty target")
	}
	if !strings.Contains(err.Error(), "target must not be empty") {
		t.Errorf("error = %q, want mention of empty target", err)
	}
}

func TestValidateExternalsTargetHasSlash(t *testing.T) {
	cfg := &Config{
		Paths: []PathSpec{
			{Path: "src", Depth: DepthInfinity, Externals: []ExternalSpec{
				{Target: "sub/dir", Depth: DepthFiles},
			}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for target with slash")
	}
	if !strings.Contains(err.Error(), "must not contain '/'") {
		t.Errorf("error = %q", err)
	}
}

func TestValidateExternalsTargetDotDot(t *testing.T) {
	cfg := &Config{
		Paths: []PathSpec{
			{Path: "src", Depth: DepthInfinity, Externals: []ExternalSpec{
				{Target: "..", Depth: DepthFiles},
			}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for target '..'")
	}
	if !strings.Contains(err.Error(), "must not be '..'") {
		t.Errorf("error = %q", err)
	}
}

func TestValidateExternalsDuplicate(t *testing.T) {
	cfg := &Config{
		Paths: []PathSpec{
			{Path: "src", Depth: DepthInfinity, Externals: []ExternalSpec{
				{Target: "lib", Depth: DepthFiles},
				{Target: "lib", Depth: DepthInfinity},
			}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for duplicate target")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("error = %q", err)
	}
}

func TestValidateExternalsWithEmptyDepth(t *testing.T) {
	cfg := &Config{
		Paths: []PathSpec{
			{Path: "src", Depth: DepthEmpty, Externals: []ExternalSpec{
				{Target: "lib", Depth: DepthFiles},
			}},
		},
	}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error for externals with depth:empty parent")
	}
	if !strings.Contains(err.Error(), "cannot declare externals") {
		t.Errorf("error = %q", err)
	}
}

func TestValidateExternalsOK(t *testing.T) {
	cfg := &Config{
		Paths: []PathSpec{
			{Path: "src", Depth: DepthInfinity, Externals: []ExternalSpec{
				{Target: "lib", Depth: DepthFiles},
			}},
		},
	}
	if err := Validate(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
