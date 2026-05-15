package jobpolicy

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestEvaluateAllowsMinimalConfigureAgentWhenEnvelopeAndEnrollmentMatch(t *testing.T) {
	now := time.Unix(1_760_000_000, 0)
	input := baseInput(now)
	input.Job.JobType = JobTypeOpenClawConfigureAgent
	input.Job.Category = CategoryOpenClaw
	input.Job.PayloadJSON = mustJSON(t, map[string]string{
		"agent_id":       "agent-1",
		"config_binding": "server-config-1",
	})
	input.Job.PayloadHash = digestHex(input.Job.PayloadJSON)

	decision := Evaluate(input)
	assertDecision(t, decision, true, ReasonOK)
}

func TestEvaluateRejectsMissingOrMismatchedPayloadHash(t *testing.T) {
	now := time.Unix(1_760_000_000, 0)

	for name, tc := range map[string]struct {
		mutate func(*EvaluationInput)
	}{
		"missing payload hash": {
			mutate: func(in *EvaluationInput) { in.Job.PayloadHash = "" },
		},
		"mismatched payload hash": {
			mutate: func(in *EvaluationInput) {
				in.Job.PayloadHash = digestHex([]byte(`{"agent_id":"agent-1","config_binding":"tampered"}`))
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			input := configureAgentInput(t, now)
			input.Job.PayloadHash = digestHex(input.Job.PayloadJSON)
			tc.mutate(&input)

			decision := Evaluate(input)
			assertDecision(t, decision, false, ReasonSchemaInvalid)
		})
	}
}

func TestEvaluateRejectsClosedSchemaAndForbiddenPayloadAuthority(t *testing.T) {
	now := time.Unix(1_760_000_000, 0)

	for name, tc := range map[string]struct {
		mutate func(*EvaluationInput)
		want   Reason
	}{
		"unknown job type": {
			mutate: func(in *EvaluationInput) { in.Job.JobType = "shell.exec" },
			want:   ReasonUnknownJobType,
		},
		"unsupported schema version": {
			mutate: func(in *EvaluationInput) { in.Job.SchemaVersion = 2 },
			want:   ReasonSchemaInvalid,
		},
		"malformed payload": {
			mutate: func(in *EvaluationInput) { in.Job.PayloadJSON = []byte(`{"agent_id":`) },
			want:   ReasonSchemaInvalid,
		},
		"extra payload field": {
			mutate: func(in *EvaluationInput) {
				in.Job.PayloadJSON = []byte(`{"agent_id":"agent-1","config_binding":"server-config-1","extra":true}`)
			},
			want: ReasonSchemaInvalid,
		},
		"shell payload authority": {
			mutate: func(in *EvaluationInput) {
				in.Job.PayloadJSON = []byte(`{"agent_id":"agent-1","config_binding":"server-config-1","shell":"/bin/sh"}`)
			},
			want: ReasonSchemaInvalid,
		},
		"argv payload authority": {
			mutate: func(in *EvaluationInput) {
				in.Job.PayloadJSON = []byte(`{"agent_id":"agent-1","config_binding":"server-config-1","argv":["restart"]}`)
			},
			want: ReasonSchemaInvalid,
		},
		"service unit payload authority": {
			mutate: func(in *EvaluationInput) {
				in.Job.PayloadJSON = []byte(`{"operation":"restart","service_unit":"evil.service"}`)
				in.Job.JobType = JobTypeServiceLifecycle
			},
			want: ReasonSchemaInvalid,
		},
		"path payload authority": {
			mutate: func(in *EvaluationInput) {
				in.Job.PayloadJSON = []byte(`{"state_key":"k","path":"/etc/passwd"}`)
				in.Job.JobType = JobTypeStateWrite
			},
			want: ReasonSchemaInvalid,
		},
		"domain payload authority": {
			mutate: func(in *EvaluationInput) {
				in.Job.PayloadJSON = []byte(`{"connection_id":"c","domain":"https://evil.example"}`)
				in.Job.JobType = JobTypePluginConfigureConnection
			},
			want: ReasonSchemaInvalid,
		},
		"credential payload authority": {
			mutate: func(in *EvaluationInput) {
				in.Job.PayloadJSON = []byte(`{"connection_id":"c","credential":"secret"}`)
				in.Job.JobType = JobTypePluginConfigureConnection
			},
			want: ReasonSchemaInvalid,
		},
	} {
		t.Run(name, func(t *testing.T) {
			input := configureAgentInput(t, now)
			tc.mutate(&input)
			decision := Evaluate(input)
			assertDecision(t, decision, false, tc.want)
		})
	}
}

func TestEvaluateRejectsLocalStateDriftBeforePolicyAuthority(t *testing.T) {
	now := time.Unix(1_760_000_000, 0)

	for name, tc := range map[string]struct {
		mutate func(*EvaluationInput)
		want   Reason
	}{
		"wrong owner": {
			mutate: func(in *EvaluationInput) { in.Job.OwnerUserID = "user-other" },
			want:   ReasonWrongOwner,
		},
		"wrong org": {
			mutate: func(in *EvaluationInput) { in.Job.OrgID = "org-other" },
			want:   ReasonWrongOrg,
		},
		"wrong enrollment": {
			mutate: func(in *EvaluationInput) { in.Job.EnrollmentID = "enroll-other" },
			want:   ReasonPolicyDenied,
		},
		"wrong device": {
			mutate: func(in *EvaluationInput) { in.Job.HelperDeviceID = "device-other" },
			want:   ReasonPolicyDenied,
		},
		"wrong credential generation": {
			mutate: func(in *EvaluationInput) { in.Job.CredentialGeneration++ },
			want:   ReasonStaleCredential,
		},
		"pending state": {
			mutate: func(in *EvaluationInput) { in.Enrollment.Status = "pending" },
			want:   ReasonPolicyDenied,
		},
		"revoked state": {
			mutate: func(in *EvaluationInput) { in.Enrollment.Revoked = true },
			want:   ReasonRevoked,
		},
		"uninstalled state": {
			mutate: func(in *EvaluationInput) { in.Enrollment.Uninstalled = true },
			want:   ReasonRevoked,
		},
		"stale credential": {
			mutate: func(in *EvaluationInput) { in.Enrollment.StaleCredential = true },
			want:   ReasonStaleCredential,
		},
		"missing category": {
			mutate: func(in *EvaluationInput) { in.Enrollment.AllowedCategories = []string{CategoryServiceLifecycle} },
			want:   ReasonPolicyDenied,
		},
		"expired job": {
			mutate: func(in *EvaluationInput) { in.Job.ExpiresAt = now.Add(-time.Second) },
			want:   ReasonPolicyDenied,
		},
	} {
		t.Run(name, func(t *testing.T) {
			input := configureAgentInput(t, now)
			tc.mutate(&input)
			decision := Evaluate(input)
			assertDecision(t, decision, false, tc.want)
		})
	}
}

func TestEvaluateInstallManifestRequiresSignedManifestArtifactAndBinding(t *testing.T) {
	now := time.Unix(1_760_000_000, 0)
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	manifestJSON, manifestDigest := signedManifest(t, priv, signedManifestSpec{
		IssuedAt:  now.Add(-time.Minute),
		ExpiresAt: now.Add(time.Hour),
		Artifacts: []ArtifactDeclaration{{
			ID:       "openclaw-plugin",
			Platform: "linux-x64",
			Version:  "1.2.3",
			SHA256:   digestHex([]byte("artifact-bytes")),
			Origin:   "https://cdn.borgee.io",
		}},
		Paths:    []PathDeclaration{{ID: "openclaw_config", Root: "/var/lib/openclaw", Mode: "write_config"}},
		Domains:  []string{"https://cdn.borgee.io"},
		Services: []ServiceDeclaration{{ID: "openclaw-user", Platform: "linux", Manager: "systemd", Unit: "openclaw.service"}},
	})

	allowed := installInput(t, now)
	allowed.TrustRoots = []ed25519.PublicKey{pub}
	allowed.Platform = "linux-x64"
	allowed.Job.ManifestDigest = manifestDigest
	allowed.Job.ManifestJSON = manifestJSON
	allowed.Job.ManifestBindingJSON = mustJSON(t, ManifestBinding{
		ManifestDigest: manifestDigest,
		ArtifactIDs:    []string{"openclaw-plugin"},
		PathIDs:        []string{"openclaw_config"},
		Domains:        []string{"https://cdn.borgee.io"},
		ServiceIDs:     []string{"openclaw-user"},
	})
	allowed.Artifacts = map[string][]byte{"openclaw-plugin": []byte("artifact-bytes")}
	allowed.Sandbox = SandboxProfile{
		WriteRoots:     []string{"/var/lib/openclaw"},
		AllowedOrigins: []string{"https://cdn.borgee.io"},
		ServiceIDs:     []string{"openclaw-user"},
	}

	assertDecision(t, Evaluate(allowed), true, ReasonOK)

	for name, tc := range map[string]struct {
		mutate func(*EvaluationInput)
		want   Reason
	}{
		"missing manifest": {
			mutate: func(in *EvaluationInput) { in.Job.ManifestJSON = nil },
			want:   ReasonManifestInvalid,
		},
		"bad signature": {
			mutate: func(in *EvaluationInput) { in.Job.ManifestJSON = corruptManifestSignature(t, in.Job.ManifestJSON) },
			want:   ReasonManifestInvalid,
		},
		"wrong trust root": {
			mutate: func(in *EvaluationInput) {
				otherPub, _, _ := ed25519.GenerateKey(rand.Reader)
				in.TrustRoots = []ed25519.PublicKey{otherPub}
			},
			want: ReasonManifestInvalid,
		},
		"manifest digest mismatch": {
			mutate: func(in *EvaluationInput) { in.Job.ManifestDigest = "sha256:" + strings.Repeat("0", 64) },
			want:   ReasonManifestInvalid,
		},
		"binding digest mismatch": {
			mutate: func(in *EvaluationInput) {
				in.Job.ManifestBindingJSON = mustJSON(t, ManifestBinding{ManifestDigest: "sha256:" + strings.Repeat("1", 64)})
			},
			want: ReasonManifestInvalid,
		},
		"artifact missing from cache": {
			mutate: func(in *EvaluationInput) { in.Artifacts = map[string][]byte{} },
			want:   ReasonArtifactInvalid,
		},
		"artifact digest mismatch": {
			mutate: func(in *EvaluationInput) { in.Artifacts = map[string][]byte{"openclaw-plugin": []byte("changed")} },
			want:   ReasonArtifactInvalid,
		},
		"unknown artifact binding": {
			mutate: func(in *EvaluationInput) {
				in.Job.ManifestBindingJSON = bindingWith(t, manifestDigest, []string{"missing"}, []string{"openclaw_config"}, []string{"https://cdn.borgee.io"}, []string{"openclaw-user"})
			},
			want: ReasonArtifactInvalid,
		},
		"unknown path binding": {
			mutate: func(in *EvaluationInput) {
				in.Job.ManifestBindingJSON = bindingWith(t, manifestDigest, []string{"openclaw-plugin"}, []string{"missing"}, []string{"https://cdn.borgee.io"}, []string{"openclaw-user"})
			},
			want: ReasonPathDenied,
		},
		"unknown domain binding": {
			mutate: func(in *EvaluationInput) {
				in.Job.ManifestBindingJSON = bindingWith(t, manifestDigest, []string{"openclaw-plugin"}, []string{"openclaw_config"}, []string{"https://evil.example"}, []string{"openclaw-user"})
			},
			want: ReasonDomainDenied,
		},
		"unknown service binding": {
			mutate: func(in *EvaluationInput) {
				in.Job.ManifestBindingJSON = bindingWith(t, manifestDigest, []string{"openclaw-plugin"}, []string{"openclaw_config"}, []string{"https://cdn.borgee.io"}, []string{"evil-service"})
			},
			want: ReasonServiceDenied,
		},
		"artifact origin not bound as allowed domain": {
			mutate: func(in *EvaluationInput) {
				in.Job.ManifestJSON, in.Job.ManifestDigest = signedManifest(t, priv, signedManifestSpec{
					IssuedAt:  now.Add(-time.Minute),
					ExpiresAt: now.Add(time.Hour),
					Artifacts: []ArtifactDeclaration{{
						ID:       "openclaw-plugin",
						Platform: "linux-x64",
						Version:  "1.2.3",
						SHA256:   digestHex([]byte("artifact-bytes")),
						Origin:   "https://evil.example",
					}},
					Paths:    []PathDeclaration{{ID: "openclaw_config", Root: "/var/lib/openclaw", Mode: "write_config"}},
					Domains:  []string{"https://cdn.borgee.io"},
					Services: []ServiceDeclaration{{ID: "openclaw-user", Platform: "linux", Manager: "systemd", Unit: "openclaw.service"}},
				})
				in.Job.ManifestBindingJSON = bindingWith(t, in.Job.ManifestDigest, []string{"openclaw-plugin"}, []string{"openclaw_config"}, []string{"https://cdn.borgee.io"}, []string{"openclaw-user"})
			},
			want: ReasonDomainDenied,
		},
		"expired manifest": {
			mutate: func(in *EvaluationInput) {
				in.Job.ManifestJSON, in.Job.ManifestDigest = signedManifest(t, priv, signedManifestSpec{IssuedAt: now.Add(-2 * time.Hour), ExpiresAt: now.Add(-time.Hour)})
				in.Job.ManifestBindingJSON = mustJSON(t, ManifestBinding{ManifestDigest: in.Job.ManifestDigest})
			},
			want: ReasonManifestInvalid,
		},
	} {
		t.Run(name, func(t *testing.T) {
			input := allowed
			tc.mutate(&input)
			decision := Evaluate(input)
			assertDecision(t, decision, false, tc.want)
		})
	}
}

func TestEvaluateDeniesPathDomainServiceAndSandboxProfileMismatch(t *testing.T) {
	now := time.Unix(1_760_000_000, 0)
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	for name, tc := range map[string]struct {
		manifest signedManifestSpec
		sandbox  SandboxProfile
		want     Reason
	}{
		"path traversal": {
			manifest: signedManifestSpec{
				IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour),
				Paths: []PathDeclaration{{ID: "openclaw_config", Root: "/var/lib/openclaw/../evil", Mode: "write_config"}},
			},
			sandbox: SandboxProfile{WriteRoots: []string{"/var/lib/openclaw"}},
			want:    ReasonPathDenied,
		},
		"relative path": {
			manifest: signedManifestSpec{
				IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour),
				Paths: []PathDeclaration{{ID: "openclaw_config", Root: "var/lib/openclaw", Mode: "write_config"}},
			},
			sandbox: SandboxProfile{WriteRoots: []string{"/var/lib/openclaw"}},
			want:    ReasonPathDenied,
		},
		"nul path": {
			manifest: signedManifestSpec{
				IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour),
				Paths: []PathDeclaration{{ID: "openclaw_config", Root: "/var/lib/openclaw\x00evil", Mode: "write_config"}},
			},
			sandbox: SandboxProfile{WriteRoots: []string{"/var/lib/openclaw"}},
			want:    ReasonPathDenied,
		},
		"missing write root capability": {
			manifest: signedManifestSpec{
				IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour),
				Paths: []PathDeclaration{{ID: "openclaw_config", Root: "/var/lib/openclaw", Mode: "write_config"}},
			},
			sandbox: SandboxProfile{},
			want:    ReasonPolicyDenied,
		},
		"local private origin": {
			manifest: signedManifestSpec{
				IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour),
				Domains: []string{"https://169.254.169.254"},
			},
			sandbox: SandboxProfile{WriteRoots: []string{"/var/lib/openclaw"}, AllowedOrigins: []string{"https://169.254.169.254"}},
			want:    ReasonDomainDenied,
		},
		"missing outbound origin capability": {
			manifest: signedManifestSpec{
				IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour),
				Domains: []string{"https://cdn.borgee.io"},
			},
			sandbox: SandboxProfile{WriteRoots: []string{"/var/lib/openclaw"}},
			want:    ReasonPolicyDenied,
		},
		"missing service capability": {
			manifest: signedManifestSpec{
				IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour),
				Services: []ServiceDeclaration{{ID: "openclaw-user", Platform: "linux", Manager: "systemd", Unit: "openclaw.service"}},
			},
			sandbox: SandboxProfile{WriteRoots: []string{"/var/lib/openclaw"}, AllowedOrigins: []string{"https://cdn.borgee.io"}},
			want:    ReasonPolicyDenied,
		},
	} {
		t.Run(name, func(t *testing.T) {
			spec := tc.manifest
			if len(spec.Artifacts) == 0 {
				spec.Artifacts = []ArtifactDeclaration{{ID: "openclaw-plugin", Platform: "linux-x64", Version: "1.2.3", SHA256: digestHex([]byte("artifact-bytes")), Origin: "https://cdn.borgee.io"}}
			}
			if len(spec.Paths) == 0 {
				spec.Paths = []PathDeclaration{{ID: "openclaw_config", Root: "/var/lib/openclaw", Mode: "write_config"}}
			}
			if len(spec.Domains) == 0 {
				spec.Domains = []string{"https://cdn.borgee.io"}
			}
			manifestJSON, manifestDigest := signedManifest(t, priv, spec)
			input := installInput(t, now)
			input.TrustRoots = []ed25519.PublicKey{pub}
			input.Platform = "linux-x64"
			input.Job.ManifestDigest = manifestDigest
			input.Job.ManifestJSON = manifestJSON
			input.Job.ManifestBindingJSON = mustJSON(t, bindingForSpec(manifestDigest, spec))
			input.Artifacts = map[string][]byte{"openclaw-plugin": []byte("artifact-bytes")}
			input.Sandbox = tc.sandbox
			decision := Evaluate(input)
			assertDecision(t, decision, false, tc.want)
		})
	}
}

func TestEvaluateValidatesServiceLifecycleWithoutExecutingServiceManager(t *testing.T) {
	now := time.Unix(1_760_000_000, 0)
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	manifestJSON, manifestDigest := signedManifest(t, priv, signedManifestSpec{
		IssuedAt:  now.Add(-time.Minute),
		ExpiresAt: now.Add(time.Hour),
		Services:  []ServiceDeclaration{{ID: "openclaw-user", Platform: "linux", Manager: "systemd", Unit: "openclaw.service"}},
	})
	input := baseInput(now)
	input.TrustRoots = []ed25519.PublicKey{pub}
	input.Platform = "linux"
	input.Job.JobType = JobTypeServiceLifecycle
	input.Job.Category = CategoryServiceLifecycle
	input.Job.PayloadJSON = mustJSON(t, map[string]string{"operation": "restart"})
	input.Job.PayloadHash = digestHex(input.Job.PayloadJSON)
	input.Job.ManifestDigest = manifestDigest
	input.Job.ManifestJSON = manifestJSON
	input.Job.ManifestBindingJSON = mustJSON(t, ManifestBinding{ManifestDigest: manifestDigest, ServiceIDs: []string{"openclaw-user"}})
	input.Enrollment.AllowedCategories = append(input.Enrollment.AllowedCategories, CategoryServiceLifecycle)
	input.Sandbox.ServiceIDs = []string{"openclaw-user"}

	assertDecision(t, Evaluate(input), true, ReasonOK)

	input.Job.PayloadJSON = mustJSON(t, map[string]string{"operation": "reload"})
	assertDecision(t, Evaluate(input), false, ReasonSchemaInvalid)
}

func TestEvaluateStateWriteRequiresWritePathModeAndWriteSandboxCapability(t *testing.T) {
	now := time.Unix(1_760_000_000, 0)
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	for name, tc := range map[string]struct {
		pathMode string
		sandbox  SandboxProfile
		want     Reason
	}{
		"read path mode with read only sandbox": {
			pathMode: "read",
			sandbox:  SandboxProfile{ReadRoots: []string{"/var/lib/borgee-helper/state"}},
			want:     ReasonPolicyDenied,
		},
		"write path mode without write sandbox capability": {
			pathMode: "write_state",
			sandbox:  SandboxProfile{ReadRoots: []string{"/var/lib/borgee-helper/state"}},
			want:     ReasonPolicyDenied,
		},
	} {
		t.Run(name, func(t *testing.T) {
			manifestJSON, manifestDigest := signedManifest(t, priv, signedManifestSpec{
				IssuedAt:  now.Add(-time.Minute),
				ExpiresAt: now.Add(time.Hour),
				Paths:     []PathDeclaration{{ID: "helper_state", Root: "/var/lib/borgee-helper/state", Mode: tc.pathMode}},
			})

			input := baseInput(now)
			input.TrustRoots = []ed25519.PublicKey{pub}
			input.Job.JobType = JobTypeStateWrite
			input.Job.Category = CategoryOpenClaw
			input.Job.PayloadJSON = mustJSON(t, map[string]string{"state_key": "openclaw/config"})
			input.Job.PayloadHash = digestHex(input.Job.PayloadJSON)
			input.Job.ManifestDigest = manifestDigest
			input.Job.ManifestJSON = manifestJSON
			input.Job.ManifestBindingJSON = mustJSON(t, ManifestBinding{ManifestDigest: manifestDigest, PathIDs: []string{"helper_state"}})
			input.Sandbox = tc.sandbox

			decision := Evaluate(input)
			assertDecision(t, decision, false, tc.want)
		})
	}
}

type signedManifestSpec struct {
	IssuedAt  time.Time
	ExpiresAt time.Time
	Artifacts []ArtifactDeclaration
	Paths     []PathDeclaration
	Domains   []string
	Services  []ServiceDeclaration
}

func configureAgentInput(t *testing.T, now time.Time) EvaluationInput {
	t.Helper()
	input := baseInput(now)
	input.Job.JobType = JobTypeOpenClawConfigureAgent
	input.Job.Category = CategoryOpenClaw
	input.Job.PayloadJSON = mustJSON(t, map[string]string{
		"agent_id":       "agent-1",
		"config_binding": "server-config-1",
	})
	input.Job.PayloadHash = digestHex(input.Job.PayloadJSON)
	return input
}

func installInput(t *testing.T, now time.Time) EvaluationInput {
	t.Helper()
	input := baseInput(now)
	input.Job.JobType = JobTypeOpenClawInstallFromManifest
	input.Job.Category = CategoryOpenClaw
	input.Job.PayloadJSON = mustJSON(t, map[string]string{"install_plan_id": "plan-1"})
	input.Job.PayloadHash = digestHex(input.Job.PayloadJSON)
	return input
}

func baseInput(now time.Time) EvaluationInput {
	return EvaluationInput{
		Now:      now,
		Platform: "linux-x64",
		Job: Job{
			JobID:                "job-1",
			OwnerUserID:          "user-1",
			OrgID:                "org-1",
			EnrollmentID:         "enroll-1",
			HelperDeviceID:       "device-1",
			CredentialGeneration: 4,
			SchemaVersion:        1,
			ExpiresAt:            now.Add(time.Hour),
		},
		Enrollment: EnrollmentState{
			OwnerUserID:          "user-1",
			OrgID:                "org-1",
			EnrollmentID:         "enroll-1",
			HelperDeviceID:       "device-1",
			CredentialGeneration: 4,
			Status:               "active",
			AllowedCategories:    []string{CategoryOpenClaw},
		},
	}
}

func signedManifest(t *testing.T, priv ed25519.PrivateKey, spec signedManifestSpec) ([]byte, string) {
	t.Helper()
	unsigned := PolicyManifest{
		ManifestVersion: 1,
		IssuedAt:        spec.IssuedAt,
		ExpiresAt:       spec.ExpiresAt,
		Artifacts:       spec.Artifacts,
		Paths:           spec.Paths,
		Domains:         spec.Domains,
		Services:        spec.Services,
	}
	canonical, err := CanonicalManifestBytes(unsigned)
	if err != nil {
		t.Fatalf("canonical manifest: %v", err)
	}
	unsigned.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, canonical))
	raw, err := json.Marshal(unsigned)
	if err != nil {
		t.Fatalf("marshal signed manifest: %v", err)
	}
	sum := sha256.Sum256(canonical)
	return raw, "sha256:" + hex.EncodeToString(sum[:])
}

func bindingWith(t *testing.T, digest string, artifactIDs, pathIDs, domains, serviceIDs []string) []byte {
	t.Helper()
	return mustJSON(t, ManifestBinding{
		ManifestDigest: digest,
		ArtifactIDs:    artifactIDs,
		PathIDs:        pathIDs,
		Domains:        domains,
		ServiceIDs:     serviceIDs,
	})
}

func bindingForSpec(digest string, spec signedManifestSpec) ManifestBinding {
	binding := ManifestBinding{ManifestDigest: digest, Domains: spec.Domains}
	for _, artifact := range spec.Artifacts {
		binding.ArtifactIDs = append(binding.ArtifactIDs, artifact.ID)
	}
	for _, path := range spec.Paths {
		binding.PathIDs = append(binding.PathIDs, path.ID)
	}
	for _, service := range spec.Services {
		binding.ServiceIDs = append(binding.ServiceIDs, service.ID)
	}
	return binding
}

func digestHex(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return raw
}

func corruptManifestSignature(t *testing.T, raw []byte) []byte {
	t.Helper()
	var manifest PolicyManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	manifest.Signature = base64.StdEncoding.EncodeToString([]byte(strings.Repeat("x", ed25519.SignatureSize)))
	out, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal corrupt manifest: %v", err)
	}
	return out
}

func assertDecision(t *testing.T, got Decision, wantAllow bool, wantReason Reason) {
	t.Helper()
	if got.Allow != wantAllow || got.Reason != wantReason {
		t.Fatalf("decision: got allow=%v reason=%s; want allow=%v reason=%s", got.Allow, got.Reason, wantAllow, wantReason)
	}
}
