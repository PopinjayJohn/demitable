// Package democampaign holds the demo campaign written by hand in phase 0
// (phase-0 question 8, "what is the schema missing?") and the projection
// test over it.
//
// The campaign itself lives in vault/ as plain markdown with YAML
// frontmatter. It is kept: it is phase 1's demo campaign.
//
// The test in projection_test.go is a small, disposable resolver for the
// visibility model of ADR-0033, ADR-0037, ADR-0040 and ADR-0048, run over
// that campaign. It is the first draft of phase 1's M5c property test: it
// resolves the same model against the same notes and asserts the result in
// both directions — that no GM-only note reaches a player, and that every
// party-visible note does.
//
// It also runs the schema as it stands today against the schema this spike
// proposes, so that every correction the campaign forced is a failing case
// with a name, rather than a paragraph of prose.
package democampaign
