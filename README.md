# flowc

`flowc` compiles flow Markdown into the graph that the Flow Tracker
Omarchy shell overlay renders, and manages a central store of projects.

One machine holds many projects; the overlay switches between them. Flows are
**not** committed into each repo — they live centrally so a person or an AI can
author and regenerate them without touching the codebase.

## Install

Prebuilt binaries are on the GitHub Releases page, or build locally:

```sh
go install github.com/madddtone/flowc/cmd/flowc@latest
# or
make install     # -> ~/.local/bin/flowc
```

## Quick start

```sh
flowc new "Checkout Service" --repo ~/code/checkout
flowc prompt checkout-service        # instructions for you or an AI
$EDITOR "$(flowc list >/dev/null; echo ~/.local/share/flow-tracker/projects/checkout-service/flow.md)"
flowc check checkout-service
flowc open checkout-service          # compile + open the canvas
flowc watch checkout-service         # recompile on save
```

## Project management

| Command | Purpose |
|---|---|
| `flowc new "<name>" [--repo PATH] [--description TEXT]` | Create a project (scaffolds `flow.md`). |
| `flowc list` | List projects with graph size and last update. |
| `flowc use <project>` | Set the active project (compiles + updates the canvas). |
| `flowc rm <project>` | Delete a project. |
| `flowc prompt <project>` | Print an AI/human authoring prompt (with the exact path). |
| `flowc guide` | Print the full authoring guide. |

## Compile / view

| Command | Purpose |
|---|---|
| `flowc compile [project\|flow.md] [-o PATH]` | Compile to `flow.json`. |
| `flowc check [project\|flow.md]` | Lint only (non-zero exit on errors). |
| `flowc open [project]` | Compile, set active, open the overlay (`--no-summon` to skip). |
| `flowc watch [project]` | Recompile on save (and on subflow edits). |
| `flowc tui [project\|flow.md]` | Interactive capture wizard. |
| `flowc init [dir]` | Scaffold a local `flow.md` (not centralised). |
| `flowc schema` | Print the terse format spec. |

Commands accept either a project name/id or a path to a `.md` file. With no
argument they operate on the active project.

## Where everything lives

```
$XDG_DATA_HOME/flow-tracker/            (default ~/.local/share/flow-tracker)
├── GUIDE.md                            authoring guide (auto-written)
├── index.json                          project list the canvas reads
└── projects/<id>/
    ├── flow.md                         ← authored source (you / the AI)
    ├── flow.json                       compiled graph (generated)
    └── meta.json                       name, repo, description, stats

$XDG_STATE_HOME/flow-tracker/active.json   which project is active (+ last error)
```

## Teaching an AI to write flows

Hand the model the output of:

```sh
flowc prompt <project>
```

It contains the exact file path, the format, the rules, and the follow-up
commands. The same information is in `flowc guide` for humans.
See [AGENTS.md](AGENTS.md).

## Format

YAML front matter for flow metadata, then one `## id` section per node with a
`yaml` block (`type`, `title`, `actor`, `code`, `routes`, ...) plus optional
`###` prose sections (`Logic`, `Requirements`, `Prerequisites`, `Inputs`,
`Outputs`, `Failure Modes`, `Notes`, ...). Full details: `flowc guide` or
[docs/flow-format.md](docs/flow-format.md).

## Layout

Positions are computed at compile time by a deterministic layered layout
(DFS back-edge detection → longest-path ranking → optional actor swimlanes →
cell placement). Cycles are allowed; back-edges are flagged so the canvas can
route them around the graph.

## License

MIT
