# AGENTS.md

Guidance for AI coding agents working with `flowc` and Flow Tracker.

## If you were asked to document a project's flow

1. Run `flowc prompt <project>` and follow its output exactly. It prints the
   **absolute path** to write and the full format.
2. Flows are **centralised** — never write `flow.md` into the project repo.
   The canonical location is
   `$XDG_DATA_HOME/flow-tracker/projects/<id>/flow.md`
   (default `~/.local/share/flow-tracker/projects/<id>/flow.md`).
3. Write the whole `flow.md` in one pass, then run
   `flowc check <project>` and fix every error (warnings worth fixing too).
4. Finish with `flowc open <project>` so the canvas shows it.

If the project does not exist yet: `flowc new "<Name>" --repo <path>`.

The complete, always-current format spec is `flowc guide` (also written to
`~/.local/share/flow-tracker/GUIDE.md`). Do not rely on a copied snapshot;
prefer `flowc guide` / `flowc prompt`.

## If you are developing flowc itself

- Layout: `cmd/flowc` (CLI), `internal/{markdown,graph,validate,layout,compile,store,state,schema,tui}`.
- Build: `make build`.  Test: `make test`.  Vet/format: `make vet` / `make fmt`.
- The format is defined in `internal/schema` (schema.go = terse spec,
  guide.go = full guide + AI prompt). Regenerate `docs/flow-format.md` with
  `make docs`.
- Keep `flowc compile` deterministic: same input must produce identical
  `flow.json` (layout is stable by design).

## If you are developing the Omarchy overlay plugin

- Plugin id `io.github.madddtone.flow-tracker`, lives at
  `~/.config/omarchy/plugins/io.github.madddtone.flow-tracker/`.
- It reads `~/.local/state/flow-tracker/active.json` (active project) and
  `~/.local/share/flow-tracker/index.json` (project list).
- `keepLoaded` overlay + third-party bar widget code only reload on
  `omarchy restart shell`.
