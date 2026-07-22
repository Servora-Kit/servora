//go:build conformance

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	examplev1 "github.com/Servora-Kit/servora/api/gen/go/servora/example/v1"
)

type resourceNameConformanceVectors struct {
	Valid []struct {
		Name   string `json:"name"`
		Tenant string `json:"tenant"`
		User   string `json:"user"`
	} `json:"valid"`
	InvalidNames []string `json:"invalidNames"`
	InvalidParts []struct {
		Tenant string `json:"tenant"`
		User   string `json:"user"`
	} `json:"invalidParts"`
}

func TestGeneratedUserNameConformance(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(filepath.Join("..", "..", "conformance", "crud", "resource_names.json"))
	if err != nil {
		t.Fatalf("read shared resource-name vectors: %v", err)
	}
	var vectors resourceNameConformanceVectors
	if err := json.Unmarshal(content, &vectors); err != nil {
		t.Fatalf("decode shared resource-name vectors: %v", err)
	}
	for _, vector := range vectors.Valid {
		parsed, err := examplev1.ParseUserName(vector.Name)
		if err != nil {
			t.Errorf("ParseUserName(%q): %v", vector.Name, err)
			continue
		}
		if parsed.Tenant != vector.Tenant || parsed.User != vector.User {
			t.Errorf("ParseUserName(%q) = %#v", vector.Name, parsed)
		}
		formatted, err := parsed.Format()
		if err != nil || formatted != vector.Name {
			t.Errorf("Format(%#v) = %q, %v", parsed, formatted, err)
		}
	}
	for _, name := range vectors.InvalidNames {
		if _, err := examplev1.ParseUserName(name); err == nil {
			t.Errorf("ParseUserName(%q) accepted invalid name", name)
		}
	}
	for _, parts := range vectors.InvalidParts {
		name := examplev1.NewUserName(parts.Tenant, parts.User)
		if _, err := name.Format(); err == nil {
			t.Errorf("Format(%#v) accepted invalid parts", name)
		}
	}
}
