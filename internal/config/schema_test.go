package config

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// TestSchemaMatchesProject keeps schema/mklv.config.schema.json, which editors
// validate against, in step with the Go types Load decodes into.
func TestSchemaMatchesProject(t *testing.T) {
	data, err := os.ReadFile("../../schema/mklv.config.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties map[string]struct {
			Enum       []string                   `json:"enum"`
			Properties map[string]json.RawMessage `json:"properties"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}

	got, want := sortedKeys(schema.Properties), jsonFields(reflect.TypeFor[Project]())
	if !slices.Equal(got, want) {
		t.Errorf("schema properties = %v, Project fields = %v", got, want)
	}
	for field, typ := range map[string]reflect.Type{
		"runtime":  reflect.TypeFor[Runtime](),
		"database": reflect.TypeFor[Database](),
		"ignore":   reflect.TypeFor[Ignore](),
	} {
		got, want := sortedKeys(schema.Properties[field].Properties), jsonFields(typ)
		if !slices.Equal(got, want) {
			t.Errorf("schema %s properties = %v, Go fields = %v", field, got, want)
		}
	}
	if got := schema.Properties["kind"].Enum; !slices.Equal(got, Kinds) {
		t.Errorf("schema kind enum = %v, Kinds = %v", got, Kinds)
	}
	if got := sortedKeys(schema.Properties["tools"].Properties); !slices.Equal(got, Tools) {
		t.Errorf("schema tools = %v, Tools = %v", got, Tools)
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func jsonFields(t reflect.Type) []string {
	var names []string
	for f := range t.Fields() {
		names = append(names, strings.Split(f.Tag.Get("json"), ",")[0])
	}
	slices.Sort(names)
	return names
}

// TestRepositoryConfig checks project-standards' own mklv.config.json parses:
// no unknown fields, a known kind and well-formed tool overrides.
func TestRepositoryConfig(t *testing.T) {
	p, err := Load("../..")
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind != "cli" || p.Name != "project-standards" {
		t.Errorf("kind, name = %q, %q; want cli, project-standards", p.Kind, p.Name)
	}
}
