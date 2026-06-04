// Package devartifacts serves a small filesystem-backed artifact + plugin-
// manifest endpoint pair on the server-go dev-stack. It exists ONLY so a
// local `docker compose up -d` dev-stack can complete an end-to-end
// openclaw.install_from_manifest job (PR #1078, blocker #6/#8) without
// reaching the unprovisioned production cdn.borgee.io. Production runs
// leave BORGEE_DEV_ARTIFACTS_DIR unset and these handlers are not mounted.
//
// Two endpoints (both unauthenticated by design — dev-stack only):
//
//	GET /dev-artifacts/{plugin}/{platform}
//	    Returns raw artifact bytes.
//
//	GET /dev-artifacts/manifests/{plugin}-{platform}.json
//	    Returns a signed api.PluginManifestPayload pointing at the bytes
//	    URL above. Signed with the same ed25519 key the rest of the
//	    server uses (BORGEE_MANIFEST_SIGNING_KEY) so install-butler's
//	    pubkey trust check passes byte-identical to the prod path.
package devartifacts

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"borgee-server/internal/api"
)

// Entry captures a registered artifact: its on-disk path, sha256, byte
// size, and the synthesized plugin-manifest payload pre-built at startup.
type Entry struct {
	PluginID string
	Platform string
	Path     string
	SHA256   string
	Size     int64
}

// Registry holds the loaded entries keyed by (pluginID, platform). Empty
// when BORGEE_DEV_ARTIFACTS_DIR is unset.
type Registry struct {
	entries map[string]Entry
	logger  *slog.Logger
}

// LoadFromDir scans `dir` for `<plugin>/<platform>` files. Each entry
// becomes a Registry record with sha256 computed at load time. Empty
// `dir` returns an empty Registry (no error — production path).
func LoadFromDir(dir string, logger *slog.Logger) (*Registry, error) {
	r := &Registry{entries: map[string]Entry{}, logger: logger}
	if strings.TrimSpace(dir) == "" {
		return r, nil
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("devartifacts: resolve abs path %q: %w", dir, err)
	}
	plugins, err := os.ReadDir(abs)
	if err != nil {
		// Missing dir is logged + non-fatal (compose may not have the
		// mount in some smoke configurations).
		if logger != nil {
			logger.Warn("devartifacts.scan: open root failed; skipping", "dir", abs, "err", err)
		}
		return r, nil
	}
	for _, pd := range plugins {
		if !pd.IsDir() || strings.HasPrefix(pd.Name(), ".") {
			continue
		}
		platformDir := filepath.Join(abs, pd.Name())
		platforms, err := os.ReadDir(platformDir)
		if err != nil {
			if logger != nil {
				logger.Warn("devartifacts.scan: open plugin subdir failed", "dir", platformDir, "err", err)
			}
			continue
		}
		for _, pf := range platforms {
			if pf.IsDir() || strings.HasPrefix(pf.Name(), ".") {
				continue
			}
			full := filepath.Join(platformDir, pf.Name())
			data, err := os.ReadFile(full)
			if err != nil {
				if logger != nil {
					logger.Warn("devartifacts.scan: read failed", "path", full, "err", err)
				}
				continue
			}
			sum := sha256.Sum256(data)
			entry := Entry{
				PluginID: pd.Name(),
				Platform: pf.Name(),
				Path:     full,
				SHA256:   hex.EncodeToString(sum[:]),
				Size:     int64(len(data)),
			}
			r.entries[key(entry.PluginID, entry.Platform)] = entry
			if logger != nil {
				logger.Info("devartifacts.register",
					"plugin", entry.PluginID,
					"platform", entry.Platform,
					"sha256", entry.SHA256,
					"size", entry.Size)
			}
		}
	}
	return r, nil
}

// Entries returns a stable-sorted slice (for tests + boot log).
func (r *Registry) Entries() []Entry {
	out := make([]Entry, 0, len(r.entries))
	for _, e := range r.entries {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].PluginID != out[j].PluginID {
			return out[i].PluginID < out[j].PluginID
		}
		return out[i].Platform < out[j].Platform
	})
	return out
}

// Get returns the entry for (pluginID, platform) — ok=false when missing.
func (r *Registry) Get(pluginID, platform string) (Entry, bool) {
	e, ok := r.entries[key(pluginID, platform)]
	return e, ok
}

// Handler is the http.Handler for the two dev endpoints. The handler is
// stateless beyond the embedded Registry + signing key.
type Handler struct {
	Registry   *Registry
	SigningKey ed25519.PrivateKey
	// ManifestURLBase is the publicly reachable origin operator-side
	// (`http://borgee-server:4900`) that gets stamped into emitted
	// PluginManifestEntry.BinaryURL fields. Empty falls back to the
	// request host.
	ManifestURLBase string
	Logger          *slog.Logger
}

// RegisterRoutes mounts the two GET endpoints on `mux`. Caller skips when
// Registry has no entries (production / smoke configs).
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	if h == nil || h.Registry == nil {
		return
	}
	mux.HandleFunc("GET /dev-artifacts/{plugin}/{platform}", h.handleArtifact)
	mux.HandleFunc("GET /dev-artifacts/manifests/{plugin}/{name}", h.handleManifest)
}

func (h *Handler) handleArtifact(w http.ResponseWriter, r *http.Request) {
	plugin := r.PathValue("plugin")
	platform := r.PathValue("platform")
	entry, ok := h.Registry.Get(plugin, platform)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Borgee-Artifact-SHA256", entry.SHA256)
	http.ServeFile(w, r, entry.Path)
}

// handleManifest serves a signed plugin-manifest payload for one
// `{plugin}/{platform}.json` artifact. The payload carries one entry,
// signed identically to api.PluginManifestHandler so install-butler's
// existing trust path consumes it byte-identical with prod.
func (h *Handler) handleManifest(w http.ResponseWriter, r *http.Request) {
	plugin := r.PathValue("plugin")
	name := r.PathValue("name")
	if !strings.HasSuffix(name, ".json") {
		http.NotFound(w, r)
		return
	}
	platform := strings.TrimSuffix(name, ".json")
	if plugin == "" || platform == "" {
		http.NotFound(w, r)
		return
	}
	entry, ok := h.Registry.Get(plugin, platform)
	if !ok {
		http.NotFound(w, r)
		return
	}
	base := h.ManifestURLBase
	if base == "" {
		// Fall back to request scheme + host so the URL is reachable from
		// the same network namespace that hit us.
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		base = scheme + "://" + r.Host
	}
	binaryURL := strings.TrimRight(base, "/") + "/dev-artifacts/" + plugin + "/" + platform
	apiEntry := api.PluginManifestEntry{
		ID:        plugin,
		Version:   "0.1.0",
		BinaryURL: binaryURL,
		SHA256:    entry.SHA256,
		Platforms: []string{platform},
	}
	apiEntry.Signature = api.SignEntry(h.SigningKey, apiEntry)
	payload := api.PluginManifestPayload{
		ManifestVersion: 1,
		IssuedAt:        0,
		ExpiresAt:       0,
		Plugins:         []api.PluginManifestEntry{apiEntry},
	}
	// Top-level signature (canonical JSON bytes of payload with empty
	// Signature). Mirrors api.PluginManifestHandler.signPayload.
	payload.Signature = ""
	canonical, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "marshal failed", http.StatusInternalServerError)
		return
	}
	if h.SigningKey != nil {
		sig := ed25519.Sign(h.SigningKey, canonical)
		payload.Signature = base64.StdEncoding.EncodeToString(sig)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func key(pluginID, platform string) string {
	return pluginID + "|" + platform
}
