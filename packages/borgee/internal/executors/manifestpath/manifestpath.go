// Package manifestpath resolves filesystem paths from the signed policy
// manifest + manifest binding carried in each leased helper job. Replaces
// the old "daemon-startup flag tells executors where to write" model: paths
// are now declared (and signed) by the server-side trust root, scoped down
// per-job via the binding, and looked up here.
//
// Contract mirrors jobpolicy.validatePaths: a manifest declares a list of
// PathDeclarations (id, root, mode); a binding lists which of those PathIDs
// the leased job is allowed to touch. Executor calls Resolve(manifest,
// binding, requiredPathID) and writes under the returned absolute root.
//
// JSON shapes are the canonical jobpolicy ones; do not invent your own.
package manifestpath

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// Errors are exported so executors can map them to terminal failure codes
// (manifest_invalid / binding_invalid / manifest_missing_path_id /
// path_escape) without string-matching.
var (
	ErrManifestParse       = errors.New("manifestpath: manifest parse failed")
	ErrBindingParse        = errors.New("manifestpath: binding parse failed")
	ErrPathIDNotInBinding  = errors.New("manifestpath: required path id not listed in binding")
	ErrPathIDNotInManifest = errors.New("manifestpath: required path id not declared in manifest")
	ErrPathNotAbsolute     = errors.New("manifestpath: declared path is not an absolute clean path")
	ErrPathEscape          = errors.New("manifestpath: relative path escapes resolved root")
)

// pathDecl mirrors jobpolicy.PathDeclaration (id+root+mode). Kept private to
// this package so no caller starts depending on the structural type.
type pathDecl struct {
	ID   string `json:"id"`
	Root string `json:"root"`
	Mode string `json:"mode"`
}

type manifest struct {
	ManifestVersion int        `json:"manifest_version"`
	Paths           []pathDecl `json:"paths"`
	// All other manifest fields are intentionally ignored — full schema +
	// signature verification lives in jobpolicy.Evaluate, which the dispatcher
	// runs BEFORE the executor. manifestpath only needs to read PathDeclarations.
}

type binding struct {
	ManifestDigest string   `json:"manifest_digest"`
	PathIDs        []string `json:"path_ids,omitempty"`
}

// Resolve parses the signed manifest + binding bytes carried in a leased
// job, asserts that requiredPathID is listed in the binding AND declared in
// the manifest, and returns the manifest-declared absolute root for that ID.
//
// Strict-decode (DisallowUnknownFields) so a server that publishes a
// superset schema does not accidentally feed the executor a path it has
// not been signed off on.
func Resolve(manifestJSON, bindingJSON []byte, requiredPathID string) (string, error) {
	if len(bytes.TrimSpace(manifestJSON)) == 0 {
		return "", fmt.Errorf("%w: empty manifest", ErrManifestParse)
	}
	if len(bytes.TrimSpace(bindingJSON)) == 0 {
		return "", fmt.Errorf("%w: empty binding", ErrBindingParse)
	}
	if strings.TrimSpace(requiredPathID) == "" {
		return "", fmt.Errorf("%w: empty required path id", ErrPathIDNotInBinding)
	}

	var b binding
	if err := decodeStrict(bindingJSON, &b); err != nil {
		return "", fmt.Errorf("%w: %v", ErrBindingParse, err)
	}
	if !contains(b.PathIDs, requiredPathID) {
		return "", fmt.Errorf("%w: %q", ErrPathIDNotInBinding, requiredPathID)
	}

	// Manifest may carry signature + extra fields not modeled here; we
	// keep DisallowUnknownFields disabled for the manifest so a fully-
	// signed manifest with artifacts/domains/services still parses. The
	// dispatcher's jobpolicy.Evaluate already validated the signature
	// against the trust root before the executor was invoked.
	var m manifest
	if err := json.Unmarshal(manifestJSON, &m); err != nil {
		return "", fmt.Errorf("%w: %v", ErrManifestParse, err)
	}
	for _, decl := range m.Paths {
		if decl.ID != requiredPathID {
			continue
		}
		root := filepath.Clean(decl.Root)
		if !filepath.IsAbs(decl.Root) || root != decl.Root || strings.ContainsRune(root, '\x00') {
			return "", fmt.Errorf("%w: id=%q root=%q", ErrPathNotAbsolute, decl.ID, decl.Root)
		}
		if hasDotDotSegment(root) {
			return "", fmt.Errorf("%w: id=%q root=%q has .. segment", ErrPathNotAbsolute, decl.ID, decl.Root)
		}
		return root, nil
	}
	return "", fmt.Errorf("%w: %q", ErrPathIDNotInManifest, requiredPathID)
}

// JoinUnderResolved appends `relPath` to the manifest-resolved root and
// rejects any escape attempt: NUL bytes, absolute relPath, `..` segments,
// or post-Clean paths that point outside resolvedRoot.
//
// Symlink-based escape (TOCTOU) is NOT covered here — the daemon's
// landlock + sandbox-exec layer is the runtime enforcement; a realpath-
// based check is tracked under issue #1028 follow-up.
func JoinUnderResolved(resolvedRoot, relPath string) (string, error) {
	if strings.ContainsRune(relPath, '\x00') {
		return "", fmt.Errorf("%w: relPath has NUL byte", ErrPathEscape)
	}
	if relPath == "" {
		return "", fmt.Errorf("%w: empty relPath", ErrPathEscape)
	}
	if filepath.IsAbs(relPath) {
		return "", fmt.Errorf("%w: relPath %q is absolute", ErrPathEscape, relPath)
	}
	for _, part := range strings.Split(filepath.ToSlash(relPath), "/") {
		if part == ".." {
			return "", fmt.Errorf("%w: relPath %q contains ..", ErrPathEscape, relPath)
		}
	}
	cleaned := filepath.Clean(relPath)
	if cleaned == "." || strings.HasPrefix(cleaned, "..") {
		return "", fmt.Errorf("%w: relPath %q resolves outside root", ErrPathEscape, relPath)
	}
	dest := filepath.Join(resolvedRoot, cleaned)
	rel, err := filepath.Rel(resolvedRoot, dest)
	if err != nil || rel == ".." || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("%w: relPath %q resolves outside %q", ErrPathEscape, relPath, resolvedRoot)
	}
	return dest, nil
}

func decodeStrict(raw []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return errors.New("extra json value")
	}
	return nil
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

func hasDotDotSegment(p string) bool {
	for _, part := range strings.Split(p, string(filepath.Separator)) {
		if part == ".." {
			return true
		}
	}
	return false
}
