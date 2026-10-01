# Flow Tracker — authoring guide

Flow Tracker renders a repo's process as an interactive flow chart. The
visualiser (an Omarchy shell overlay) draws a compiled graph; you author the
graph as ordinary Markdown. flowc is the compiler in between.

    flow.md  ──flowc compile──▶  flow.json  ──▶  canvas overlay
       ▲
       └── you / your AI edit this

## Where flows live

Flow definitions are CENTRALISED, not committed to each project repo. Everything
lives under the Flow Tracker data directory:

    $XDG_DATA_HOME/flow-tracker/projects/<id>/flow.md     ← author this
    $XDG_DATA_HOME/flow-tracker/projects/<id>/flow.json   ← compiled (do not edit)
    $XDG_DATA_HOME/flow-tracker/projects/<id>/meta.json   ← name/repo/stats
    $XDG_DATA_HOME/flow-tracker/index.json                ← canvas project list

(Default: ~/.local/share/flow-tracker/...)

So one machine holds many projects side by side and the canvas can switch
between them. A project can record the repo it documents in meta.json, but the
flow.md itself is never placed in the repo.

## Project workflow

    flowc new "Checkout Service" --repo ~/code/checkout   # create a project
    flowc list                                            # see all projects
    flowc use checkout-service                            # make it active
    flowc prompt checkout-service                         # instructions for an AI
    flowc open checkout-service                           # compile + open the canvas
    flowc watch checkout-service                          # recompile on every save

`flowc prompt <project>` prints the exact path and rules; give that output
to your AI agent and it can write flow.md unaided.

## The file format

```
---
flow: Checkout Service       # display name
start: cart                  # entry node id
actors: [customer, system]   # optional swimlane order
---

## cart                      # node id (must be unique)
```yaml
type: start                  # start|end|step|decision|subflow|external|parallel|join
title: Cart Review           # display label (defaults to the id)
actor: customer              # optional swimlane
code: src/cart.ts:42         # optional file:line reference
tags: [p0, frontend]         # optional
routes:                      # outgoing edges
  - to: validate
```
### Logic
Customer reviews the items in their cart.

### Prerequisites
- Customer is authenticated
- Cart is not empty
```

Rules of thumb:

- Front matter is optional. Node sections are level-2 headings (`## id`).
- Each node's structured fields go in ONE ```yaml block. Long-form text
  goes under level-3 headings (`### Logic`, `### Requirements`, ...).
- `routes` is how you connect nodes. `to` must match another node id.
- Use `type: decision` for branches (2+ routes, each with a `when:`).
- Use `type: subflow` with `flow: ./other.flow.md` to nest a flow. flowc
  compiles subflows recursively (to a JSON file beside each) and the canvas
  drills into them: double-click the node, press Enter, or use the inspector
  button; Backspace/Esc returns to the parent.
- Cycles/retries are allowed; they are drawn routed around the graph.

## Node types

| type       | meaning                                             |
|------------|-----------------------------------------------------|
| start      | entry point (one per flow)                          |
| end        | terminal step                                       |
| step       | a normal action                                     |
| decision   | a branch; give it 2+ routes with `when:` guards |
| subflow    | expands into another flow.md via `flow:`        |
| external   | a system/service outside your control               |
| parallel   | fan-out to parallel work                            |
| join       | where parallel branches merge                       |

## Prose sections (any label is kept)

Recognised and shown specially in the inspector: Logic, Requirements,
Prerequisites, Inputs, Outputs, Failure Modes, Notes. Lists start with "- ".
Any other `### Label` is preserved verbatim.

## Validation

`flowc check <project>` reports:

- errors: duplicate ids, routes to unknown nodes
- warnings: unreachable nodes, dead ends, decision nodes with <2 routes,
  subflow files that don't exist

Fix errors before the canvas will render; warnings are advisory.

## Complete example

See `flowc init` for a scaffold, or open the sample written by `flowc new`.

## For AI agents

1. Run `flowc prompt <project>`; it prints the target file path and format.
2. Write the whole flow.md in one pass. Prefer stable, short node ids.
3. Run `flowc check <project>`; fix every error and sensible warning.
4. Run `flowc open <project>` so the canvas shows the result.
