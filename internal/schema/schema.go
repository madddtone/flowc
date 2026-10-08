// Package schema holds the authoring spec and the scaffold used by
// `flowc schema` and `flowc init`.
package schema

import "strings"

// SampleFor returns the starter flow with the project name substituted.
func SampleFor(name string) string {
	if strings.TrimSpace(name) == "" {
		return Sample
	}
	return strings.Replace(Sample, "flow: Checkout Flow", "flow: "+yamlSafe(name), 1)
}

func yamlSafe(s string) string {
	if strings.ContainsAny(s, ":#{}[]&*?|<>=!%@`\"'") {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return s
}

// Schema is the human/AI-facing format specification printed by `flowc schema`.
const Schema = `# flow.md format (flowc v1)

A flow is one Markdown file. By default flowc keeps it centralised at
$XDG_DATA_HOME/flow-tracker/projects/<id>/flow.md, but any .md path works.

## Front matter (optional)
---
flow: Checkout Flow        # display name
start: cart                # entry node id (defaults to first type:start node)
actors: [customer, system] # optional swimlane order
---

## Nodes
Each node is a level-2 heading whose text is the node id:

## cart
` + "```yaml" + `
type: step                 # start|end|step|decision|subflow|external|parallel|join|table
title: Cart Review         # display label (defaults to id)
actor: customer            # optional; groups into a swimlane
owner: team-checkout       # optional
status: todo               # optional
priority: p0               # optional
code: src/cart.ts:42       # optional file:line reference
tags: [frontend, p0]       # optional
flow: ./auth.flow.md       # required for type: subflow
routes:                    # outgoing edges (this is what branches)
  - to: payment
    when: cart valid       # optional guard/condition
    label: happy path      # optional edge label (alias: data)
  - to: cart-error
    when: cart invalid
` + "```" + `

## Tables (type: table)
For data-flow diagrams. A table node draws a header and its columns; the canvas
shows the first 5 (primary/foreign keys first) and the inspector shows them all.
Columns are written as mappings, or as a compact string:

## orders
` + "```yaml" + `
type: table
title: orders
schema: public             # optional
columns:
  - name: id               # structured form
    type: bigint
    pk: true
  - "user_id bigint FK users.id"   # compact: name [type] [PK] [FK ref] [NOT NULL]
  - name: total
    type: numeric
    nullable: false
routes:
  - to: charge
    data: order total      # data-flow alias for label
` + "```" + `

## Prose sections (optional, any label)
Long-form text is captured under level-3 headings. Recognized labels are
rendered specially in the canvas inspector; any other label is kept verbatim.

### Logic
### Requirements
### Prerequisites
### Inputs
### Outputs
### Failure Modes
### Notes

Lists are lines beginning with "- ". Everything else is plain text/Markdown.

## Rules
- Node ids must be unique and match route targets exactly.
- Exactly one entry point is recommended (front-matter start or a type:start node).
- A decision node should have at least two routes.
- A node with no routes is a dead end (use type: end to silence the warning).
- Cycles are allowed; back-edges are drawn routed around the graph.
- Referenced subflow files are relative to the current flow.md.
- Use ` + "`type: table`" + ` for data stores in data-flow diagrams; mark ` + "`pk`" + `/` + "`fk`" + `.
  The canvas shows up to 5 key columns; the inspector shows every column.
`

// Sample is written by `flowc init` when no flow.md exists.
const Sample = `---
flow: Checkout Flow
start: cart
actors: [customer, system]
---

## cart
` + "```yaml" + `
type: start
title: Cart Review
actor: customer
routes:
  - to: validate
` + "```" + `
### Logic
Customer reviews the items in their cart.

### Prerequisites
- Customer is authenticated
- Cart is not empty

## validate
` + "```yaml" + `
type: decision
title: Cart Valid?
actor: system
code: src/cart/validate.ts:18
routes:
  - to: charge
    when: valid
  - to: cart
    when: invalid
` + "```" + `
### Logic
Validate stock and pricing before charging.

### Failure Modes
- Inventory service times out

## charge
` + "```yaml" + `
type: step
title: Charge Payment
actor: system
code: src/payments/charge.ts:52
routes:
  - to: fulfill
` + "```" + `
### Requirements
- A valid payment method on file

### Inputs
- cart total

### Outputs
- payment intent id

## fulfill
` + "```yaml" + `
type: subflow
title: Fulfillment
flow: ./fulfillment.flow.md
actor: system
routes:
  - to: done
` + "```" + `
### Notes
Handed off to the warehouse system.

## done
` + "```yaml" + `
type: end
title: Order Complete
` + "```" + `
### Outputs
- confirmation email
`
