# project-standards

The standards every project follows regardless of language, and the automation
that keeps projects in line with them. The design is in [`docs/adr/`](docs/adr/)
and the vocabulary in [`CONTEXT.md`](CONTEXT.md).

## What a project gets

- **Shared checks:** formatting, build, vet, lint and known vulnerabilities
  (Go), Markdown, spelling, secrets and commit messages, all Go tools pinned in
  [`ci/tools.go`](ci/tools.go).
- **Generated tool configs:** `.golangci.yaml`, `.markdownlint-cli2.yaml`,
  `.gitleaks.toml` and `.commitlint.yaml`, written from the base configs in
  [`templates/base/`](templates/base/) merged with the project's overrides,
  and for a Go project of kind cli a `.goreleaser.yaml` that builds its
  `name` from its `entrypoint`'s directory, or `cmd/<name>`. Don't edit them;
  edit `mklv.config.json`.
- **One CI/CD workflow:** [`cicd.yaml`](.github/workflows/cicd.yaml) checks,
  releases and deploys every project, chosen by its languages and `kind`.
- **Hooks:** the base [`lefthook.jsonc`](lefthook.jsonc) runs the full check on
  every commit and lints the commit message.
- **Conformance:** a weekly workflow keeps the standard files current in every
  project with the `mklv-conform` topic.

## Using it

### Go projects

Import the `ci` package from the project's magefiles:

```go
//go:build mage

package main

import (
 //mage:import ci
 "github.com/dmikalova/project-standards/ci"
)

func init() {
 // Optional: extra build steps and 100% coverage areas.
 ci.ExtraBuilds = nil
 ci.CoverGates = nil
}
```

Validate locally with `mage ci:fix && mage ci:check`.

### Other projects

Install the binary (dotfiles does this):

```bash
GOBIN=~/.local/bin go install github.com/dmikalova/project-standards/cmd/project-standards@latest
```

Validate locally with `project-standards fix && project-standards check`.

### Every project

- `lefthook.jsonc` includes the base config:

  ```jsonc
  {
    "remotes": [
      {
        "configs": ["lefthook.jsonc"],
        "git_url": "https://github.com/dmikalova/project-standards",
        "ref": "main",
        "refetch": true
      }
    ]
  }
  ```

  Run `lefthook install` after cloning.

- `.github/workflows/cicd.yaml` calls
  `dmikalova/project-standards/.github/workflows/cicd.yaml@main`.
- `mklv.config.json` is optional. It declares the `kind` (cli, cloudrun, library
  or infra), deploy settings, tool overrides and ignore-file additions; see
  [`schema/mklv.config.schema.json`](schema/mklv.config.schema.json).

`project-standards conform` writes all of these from the templates, and the
weekly conformance workflow runs it for opted-in projects.

## Database schema deployment

Cloud Run projects with `db/schema.hcl` use Atlas by default. A project can
select Ptah Compat and supply HCL variables in `mklv.config.json`:

```json
{
  "database": {
    "tool": "ptah",
    "vars": { "app_role": "tasks-role" }
  }
}
```

The reusable workflow pins the Ptah release and archive SHA-256. It installs
`ptah-compat` as `atlas`, keeping the existing command interface. Ptah previews
HCL through `schema apply --dry-run` and applies the same schema without a dev
database. Atlas keeps its existing `schema diff` and dev-database path.
Both tools receive the same variables for preview and apply, and both retain
the existing guard that stops deployment on destructive schema changes.
Project callers stay generated; do not edit `.github/workflows/cicd.yaml` to
select the database tool.
