package config

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestDatabaseConfig(t *testing.T) {
	data, err := os.ReadFile("../../schema/mklv.config.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("config.json", doc); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile("config.json")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, source string
		valid        bool
	}{
		{"unchanged default", `{}`, true},
		{"empty database", `{"database":{}}`, true},
		{"explicit Atlas", `{"database":{"tool":"atlas"}}`, true},
		{"Ptah variables", `{"database":{"tool":"ptah","vars":{"app_role":"tasks-role"}}}`, true},
		{"literal variable value", `{"database":{"vars":{"value":"a b\n$(false)"}}}`, true},
		{"unknown tool", `{"database":{"tool":"other"}}`, false},
		{"unknown database field", `{"database":{"tools":"ptah"}}`, false},
		{"non-string variable", `{"database":{"vars":{"role":1}}}`, false},
		{"empty variable name", `{"database":{"vars":{"":"x"}}}`, false},
		{"equals in name", `{"database":{"vars":{"a=b":"x"}}}`, false},
		{"NUL in name", `{"database":{"vars":{"a\u0000b":"x"}}}`, false},
		{"NUL in value", `{"database":{"vars":{"role":"a\u0000b"}}}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := Parse([]byte(tt.source))
			if (err == nil) != tt.valid {
				t.Fatalf("Parse error = %v, valid = %v", err, tt.valid)
			}
			var input any
			if err := json.Unmarshal([]byte(tt.source), &input); err != nil {
				t.Fatal(err)
			}
			if err := schema.Validate(input); (err == nil) != tt.valid {
				t.Fatalf("schema error = %v, valid = %v", err, tt.valid)
			}
			if tt.name == "Ptah variables" && !reflect.DeepEqual(p.Database, &Database{Tool: "ptah", Vars: map[string]string{"app_role": "tasks-role"}}) {
				t.Fatalf("database = %#v", p.Database)
			}
		})
	}
}
