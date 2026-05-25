package helpermanifest

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestLinuxDigest_Stable — boot-time computed package-level digest must
// match a fresh recomputation. Anchors the contract that helper_jobs
// row.manifest_digest stays valid across server restarts (since the
// canonical body itself is deterministic).
func TestLinuxDigest_Stable(t *testing.T) {
	fresh, err := Digest(BuildLinux())
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	if fresh != LinuxDigest {
		t.Fatalf("LinuxDigest %q != fresh recompute %q", LinuxDigest, fresh)
	}
	if !strings.HasPrefix(fresh, "sha256:") {
		t.Fatalf("digest must use sha256: prefix, got %q", fresh)
	}
}

// TestDarwinDigest_Stable — PR-4 final amend mirror of the Linux digest
// stability check. macOS manifest body has different paths + service
// Manager, so the digest is distinct from Linux but equally
// deterministic across server reboots.
func TestDarwinDigest_Stable(t *testing.T) {
	fresh, err := Digest(BuildDarwin())
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	if fresh != DarwinDigest {
		t.Fatalf("DarwinDigest %q != fresh recompute %q", DarwinDigest, fresh)
	}
	if !strings.HasPrefix(fresh, "sha256:") {
		t.Fatalf("digest must use sha256: prefix, got %q", fresh)
	}
}

// TestCanonicalDigest_DiffersByPlatform — Linux digest must NOT equal
// Darwin digest, otherwise the daemon's manifest-digest-vs-binding
// check cannot reject cross-platform manifest delivery. Daemon binds
// each enrollment to one platform via the WS upgrade header; server
// emits the matching manifest; the digest distinguishes them so a
// daemon that somehow received the wrong platform's manifest gets
// ReasonManifestInvalid silently.
func TestCanonicalDigest_DiffersByPlatform(t *testing.T) {
	linux, err := CanonicalDigest(PlatformLinux)
	if err != nil {
		t.Fatalf("linux digest: %v", err)
	}
	darwin, err := CanonicalDigest(PlatformDarwin)
	if err != nil {
		t.Fatalf("darwin digest: %v", err)
	}
	if linux == darwin {
		t.Fatalf("linux + darwin digests must differ, both are %q", linux)
	}
}

// TestCanonicalManifest_DeclaresAllRequiredIDs — both platforms must
// declare every PathID / ServiceID referenced by the store's binding
// switch in helper_job_queries.go. Drift = daemon rejects every leased
// job for that platform with ReasonPathDenied / ServiceDenied.
func TestCanonicalManifest_DeclaresAllRequiredIDs(t *testing.T) {
	requiredPathIDs := []string{
		PathIDOpenClawInstall, PathIDOpenClawAgentConfig,
		PathIDBorgeePluginConfig, PathIDBorgeeStateConfig,
		PathIDHelperState, PathIDHelperRuntime,
	}
	requiredServiceIDs := []string{ServiceIDOpenClawUser, ServiceIDBorgeeHelper}

	for _, platform := range []Platform{PlatformLinux, PlatformDarwin} {
		manifest, err := CanonicalManifest(platform)
		if err != nil {
			t.Fatalf("%s: CanonicalManifest: %v", platform, err)
		}
		pathIDs := map[string]bool{}
		for _, p := range manifest.Paths {
			pathIDs[p.ID] = true
		}
		for _, id := range requiredPathIDs {
			if !pathIDs[id] {
				t.Fatalf("%s missing PathID %q", platform, id)
			}
		}
		svcIDs := map[string]bool{}
		for _, s := range manifest.Services {
			svcIDs[s.ID] = true
		}
		for _, id := range requiredServiceIDs {
			if !svcIDs[id] {
				t.Fatalf("%s missing ServiceID %q", platform, id)
			}
		}
		if len(manifest.Artifacts) == 0 || manifest.Artifacts[0].ID != ArtifactIDOpenClawPlugin {
			t.Fatalf("%s missing %s artifact", platform, ArtifactIDOpenClawPlugin)
		}
	}
}

// TestBuildLinux_AllRequiredIDsDeclared — locks the path/service/artifact
// IDs that store/helper_job_queries.go's binding switch references.
// Drift here = daemon rejects every leased job with ReasonPathDenied /
// ServiceDenied / ArtifactInvalid.
func TestBuildLinux_AllRequiredIDsDeclared(t *testing.T) {
	m := BuildLinux()
	pathIDs := map[string]bool{}
	for _, p := range m.Paths {
		pathIDs[p.ID] = true
	}
	required := []string{
		PathIDOpenClawInstall, PathIDOpenClawAgentConfig,
		PathIDBorgeePluginConfig, PathIDBorgeeStateConfig,
		PathIDHelperState, PathIDHelperRuntime,
	}
	for _, id := range required {
		if !pathIDs[id] {
			t.Fatalf("BuildLinux missing PathID %q", id)
		}
	}
	svcIDs := map[string]bool{}
	for _, s := range m.Services {
		svcIDs[s.ID] = true
	}
	for _, id := range []string{ServiceIDOpenClawUser, ServiceIDBorgeeHelper} {
		if !svcIDs[id] {
			t.Fatalf("BuildLinux missing ServiceID %q", id)
		}
	}
	if len(m.Artifacts) == 0 || m.Artifacts[0].ID != ArtifactIDOpenClawPlugin {
		t.Fatalf("BuildLinux missing %s artifact", ArtifactIDOpenClawPlugin)
	}
}

// TestBuildDarwin_PathRootsMatchSetupConstants — locks the Darwin path
// roots against the setup.go macOS constants. The manifest is the
// source of truth: if setup.go changes a darwin* path constant, this
// test fails until either the manifest catches up or setup.go is
// reverted. Cross-checked at PR-4 final amend time against constants
// darwinRuntimeDir + darwinAppSupport + darwinStateRoot.
func TestBuildDarwin_PathRootsMatchSetupConstants(t *testing.T) {
	want := map[string]string{
		PathIDOpenClawInstall:     "/usr/local/libexec/borgee/openclaw",
		PathIDOpenClawAgentConfig: "/Library/Application Support/Borgee/openclaw",
		PathIDBorgeePluginConfig:  "/Library/Application Support/Borgee/plugins",
		PathIDBorgeeStateConfig:   "/Library/Application Support/Borgee/state",
		PathIDHelperState:         "/Library/Application Support/Borgee",
		PathIDHelperRuntime:       "/usr/local/libexec/borgee",
	}
	got := map[string]string{}
	for _, p := range BuildDarwin().Paths {
		got[p.ID] = p.Root
	}
	for id, expected := range want {
		if got[id] != expected {
			t.Fatalf("Darwin path %q = %q, want %q", id, got[id], expected)
		}
	}
}

// TestBuildDarwin_ServicesUseLaunchd — Manager + Unit per launchd
// convention. systemd labels would break the daemon's executor: the
// service.lifecycle executor on Darwin only knows launchctl.
func TestBuildDarwin_ServicesUseLaunchd(t *testing.T) {
	wantServices := map[string]struct {
		manager string
		unit    string
	}{
		ServiceIDOpenClawUser: {"launchd", "cloud.borgee.openclaw"},
		ServiceIDBorgeeHelper: {"launchd", "cloud.borgee.host-bridge"},
	}
	got := map[string]ServiceDeclaration{}
	for _, s := range BuildDarwin().Services {
		got[s.ID] = s
	}
	for id, expected := range wantServices {
		if got[id].Manager != expected.manager {
			t.Fatalf("Darwin service %q Manager = %q, want %q", id, got[id].Manager, expected.manager)
		}
		if got[id].Unit != expected.unit {
			t.Fatalf("Darwin service %q Unit = %q, want %q", id, got[id].Unit, expected.unit)
		}
		if got[id].Platform != "darwin" {
			t.Fatalf("Darwin service %q Platform = %q, want %q", id, got[id].Platform, "darwin")
		}
	}
}

// TestParsePlatform — daemon-supplied platform tokens map to typed
// enums. Empty + unknown → ok=false so the WS upgrade handler can
// close 4002 unsupported_platform.
func TestParsePlatform(t *testing.T) {
	cases := []struct {
		token string
		want  Platform
		ok    bool
	}{
		{"linux", PlatformLinux, true},
		{"darwin", PlatformDarwin, true},
		{"", "", false},
		{"windows", "", false},
		{"LINUX", "", false}, // case-sensitive: daemon sends lower-case runtime.GOOS
	}
	for _, tc := range cases {
		got, ok := ParsePlatform(tc.token)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("ParsePlatform(%q) = (%q, %v), want (%q, %v)", tc.token, got, ok, tc.want, tc.ok)
		}
	}
	// String() round-trip — covers the simple receiver method.
	for _, p := range []Platform{PlatformLinux, PlatformDarwin} {
		if p.String() != string(p) {
			t.Fatalf("(%q).String() = %q, want %q", p, p.String(), string(p))
		}
	}
}

// TestCanonicalBytes_StripsSignature — the canonical-bytes contract
// must produce identical output regardless of whether Signature is set.
// Daemon-side jobpolicy.CanonicalManifestBytes does the same; any drift
// here breaks signature verification silently.
func TestCanonicalBytes_StripsSignature(t *testing.T) {
	m := BuildLinux()
	withoutSig, err := CanonicalBytes(m)
	if err != nil {
		t.Fatalf("without sig: %v", err)
	}
	m.Signature = "fake-signature"
	withSig, err := CanonicalBytes(m)
	if err != nil {
		t.Fatalf("with sig: %v", err)
	}
	if string(withoutSig) != string(withSig) {
		t.Fatalf("canonical bytes differ after signature set:\n a: %s\n b: %s", withoutSig, withSig)
	}
	// Canonical output is parseable as JSON.
	var parsed map[string]any
	if err := json.Unmarshal(withoutSig, &parsed); err != nil {
		t.Fatalf("canonical bytes not valid JSON: %v", err)
	}
	if parsed["manifest_version"] != float64(1) {
		t.Fatalf("manifest_version not 1: %v", parsed["manifest_version"])
	}
}
