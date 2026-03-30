// Package structuredpath owns canonical structured-edit path handling.
//
// Decision note:
//
// Deck standardizes internal structured-edit paths on JSON Pointer. The
// package still accepts legacy dotted, bracketed, and step-id-oriented alias
// inputs at the boundary so existing ask repair and refine flows continue to
// work, but every successful parse is normalized to canonical JSON Pointer.
//
// We intentionally keep this parser in-house for now instead of wrapping a
// separate JSON Pointer or JSONPath library. The remaining behavior we need is
// small, deterministic, and tightly coupled to deck's compatibility aliases.
// If future requirements grow beyond exact single-location edits, revisit this
// decision and evaluate replacing only the canonical pointer parsing layer with
// a library-backed implementation.
package structuredpath
