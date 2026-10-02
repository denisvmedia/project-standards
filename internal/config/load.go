package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// FileName is the one hand-edited configuration file in a project.
const FileName = "mklv.config.json"

// Kinds lists the valid values of the kind field (ADR 0006).
var Kinds = []string{"cli", "cloudrun", "library", "infra"}

// Tools lists the tools whose base configs a project can override under
// tools.<name>.
var Tools = []string{"commitlint", "gitleaks", "golangci", "markdownlint", "misspell"}

// Project is a parsed mklv.config.json. Every field is optional, and a project
// without the file gets the zero Project.
type Project struct {
	// Schema is the optional $schema reference editors use for completion.
	Schema string `json:"$schema,omitempty"`
	// Name identifies the app: the image tag and the Cloud Run service name.
	Name string `json:"name,omitempty"`
	// Entrypoint is the file the app is built from.
	Entrypoint string `json:"entrypoint,omitempty"`
	// Runtime holds the deployed service's runtime settings.
	Runtime *Runtime `json:"runtime,omitempty"`
	// Kind is what the project produces and how it ships: one of Kinds.
	Kind string `json:"kind,omitempty"`
	// Database selects the Cloud Run schema tool and HCL variables.
	Database *Database `json:"database,omitempty"`
	// Tools maps a tool name, one of Tools, to its override tree.
	Tools map[string]any `json:"tools,omitempty"`
	// Ignore holds additions to the universal ignore files.
	Ignore *Ignore `json:"ignore,omitempty"`
}

// Database configures schema deployment. An empty Tool keeps Atlas.
type Database struct {
	Tool string            `json:"tool,omitempty"`
	Vars map[string]string `json:"vars,omitempty"`
}

func (d *Database) validate() error {
	if d == nil {
		return nil
	}
	if d.Tool != "" && d.Tool != "atlas" && d.Tool != "ptah" {
		return fmt.Errorf("database.tool %q must be atlas or ptah", d.Tool)
	}
	for key, value := range d.Vars {
		if key == "" || strings.ContainsAny(key, "=\x00") || strings.ContainsRune(value, '\x00') {
			return errors.New("database.vars requires nonempty names without '=' and values without NUL bytes")
		}
	}
	return nil
}

// Runtime is the deployed service's runtime settings.
type Runtime struct {
	Port            int    `json:"port,omitempty"`
	HealthCheckPath string `json:"healthCheckPath,omitempty"`
}

// Ignore holds project-specific additions to the universal .gitignore and
// .dockerignore (ADR 0007).
type Ignore struct {
	Git    []string `json:"git,omitempty"`
	Docker []string `json:"docker,omitempty"`
}

// Load reads dir/mklv.config.json. A missing file is not an error: it returns
// the zero Project, which gets every base config unchanged.
func Load(dir string) (*Project, error) {
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if errors.Is(err, fs.ErrNotExist) {
		return &Project{}, nil
	}
	if err != nil {
		return nil, err
	}
	p, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", FileName, err)
	}
	return p, nil
}

// Parse decodes and validates the contents of a mklv.config.json. Unknown
// fields, an unknown kind, an unknown tool, and a tool override that is not an
// object are errors.
func Parse(data []byte) (*Project, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	dec.UseNumber()
	var p Project
	if err := dec.Decode(&p); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, errors.New("unexpected data after the top-level object")
	}
	if p.Kind != "" && !slices.Contains(Kinds, p.Kind) {
		return nil, fmt.Errorf("kind %q is not one of %v", p.Kind, Kinds)
	}
	if err := p.Database.validate(); err != nil {
		return nil, err
	}
	for name, v := range p.Tools {
		if !slices.Contains(Tools, name) {
			return nil, fmt.Errorf("tools.%s: unknown tool; known tools are %v", name, Tools)
		}
		if _, ok := v.(map[string]any); !ok {
			return nil, fmt.Errorf("tools.%s: must be an object of overrides", name)
		}
		p.Tools[name] = Normalize(v)
	}
	return &p, nil
}

// Override returns the project's override tree for a tool. A tool without one
// gets an empty object, which merges to the base config unchanged.
func (p *Project) Override(tool string) any {
	if v, ok := p.Tools[tool]; ok {
		return v
	}
	return map[string]any{}
}
