// Package worker is the embeddable lowcode-faas runtime SDK.
//
// Meta (cmd/meta) owns Action CRUD and TS→JS compile. Your service embeds this
// package, configures LOWCODE_FAAS_META_URL, registers optional host binders,
// and serves POST /api/actions/{name}/invoke.
//
// See examples/worker-embed for a complete process.
package worker
