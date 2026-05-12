// Package datalayer — DL-1 §0 principle ①: "4 interfaces byte-identical with
// blueprint §4 B table"
// (Storage / Presence / EventBus / Repository).
//
// v1 policy: interface only. The 4 interfaces use the existing implementations
// (SQLite gorm + in-memory map + DB blob + in-process map). This adds the
// interface seam needed for future implementation swaps without changing the
// implementation now.
//
// Reference: `docs/implementation/modules/dl-1-spec.md` v0.
package datalayer
