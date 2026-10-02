package config

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// Execute the workflow's actual deployment shell, with only the database CLI
// replaced. This catches a guard or variable path that differs between tools.
func TestDatabaseWorkflow(t *testing.T) {
	source, err := os.ReadFile("../../.github/workflows/cicd.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct{ Steps []struct{ Name, Run string } }
	}
	if err := yaml.Unmarshal(source, &workflow); err != nil {
		t.Fatal(err)
	}
	var script string
	for _, step := range workflow.Jobs["deploy-cloudrun"].Steps {
		if step.Name == "DB Schema Apply" {
			script = step.Run
		}
	}
	if script == "" {
		t.Fatal("schema deployment step not found")
	}
	tests := []struct {
		name, tool, plan, config string
		fail                     bool
		calls                    int
	}{
		{"Atlas unchanged", "atlas", "CREATE TABLE example(id int);", "", false, 2},
		{"Ptah production role", "ptah", "CREATE POLICY tenant ON tasks;", `{"database":{"tool":"ptah","vars":{"app_role":"tasks-role"}}}`, false, 2},
		{"literal variables", "ptah", "", `{"database":{"vars":{"value":"a b\n$(touch injected)"}}}`, false, 2},
		{"Atlas destructive guard", "atlas", "DROP TABLE example;", "", true, 1},
		{"Ptah destructive guard", "ptah", "DROP TABLE example;", "", true, 1},
		{"invalid tool", "other", "", "", true, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			binary := filepath.Join(dir, "atlas")
			fake := "#!/bin/bash\nprintf '%s\\0' \"$@\" >> \"$CALLS\"\nprintf '\\n' >> \"$CALLS\"\nprintf '%s\\n' \"$PLAN\"\n"
			if err := os.WriteFile(binary, []byte(fake), 0o755); err != nil {
				t.Fatal(err)
			}
			if tt.config != "" {
				if err := os.WriteFile(filepath.Join(dir, FileName), []byte(tt.config), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			calls := filepath.Join(dir, "calls")
			// Each test uses its own plan file, just as each workflow gets a fresh runner.
			run := strings.ReplaceAll(script, "/tmp/migration.sql", filepath.Join(dir, "migration.sql"))
			cmd := exec.Command("bash", "-eo", "pipefail", "-c", run)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"), "URL=postgres://test/db", "SCHEMA=tasks", "TOOL="+tt.tool, "RUNNER_TEMP="+dir, "CALLS="+calls, "PLAN="+tt.plan)
			output, err := cmd.CombinedOutput()
			if (err != nil) != tt.fail {
				t.Fatalf("command error = %v; output: %s", err, output)
			}
			recorded, readErr := os.ReadFile(calls)
			if tt.calls == 0 {
				if !os.IsNotExist(readErr) {
					t.Fatalf("unexpected database command: %s, %v", recorded, readErr)
				}
				return
			}
			if readErr != nil {
				t.Fatal(readErr)
			}
			if got := strings.Count(string(recorded), "schema\x00apply\x00") + strings.Count(string(recorded), "schema\x00diff\x00"); got != tt.calls {
				t.Fatalf("calls = %d, want %d: %q", got, tt.calls, recorded)
			}
			if tt.tool == "ptah" && !bytes.HasPrefix(recorded, []byte("schema\x00apply\x00--dry-run\x00")) {
				t.Fatalf("wrong Ptah preview: %q", recorded)
			}
			if tt.tool == "atlas" && !bytes.HasPrefix(recorded, []byte("schema\x00diff\x00")) {
				t.Fatalf("wrong Atlas preview: %q", recorded)
			}
			if tt.name == "Ptah production role" && strings.Count(string(recorded), "--var\x00app_role=tasks-role\x00") != 2 {
				t.Fatalf("production role missing: %q", recorded)
			}
			if tt.name == "literal variables" {
				if strings.Count(string(recorded), "--var\x00value=a b\n$(touch injected)\x00") != 2 {
					t.Fatalf("variable was split: %q", recorded)
				}
				if _, err := os.Stat(filepath.Join(dir, "injected")); !os.IsNotExist(err) {
					t.Fatal("variable executed shell code")
				}
			}
		})
	}
}
