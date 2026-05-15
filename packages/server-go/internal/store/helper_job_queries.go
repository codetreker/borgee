package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"borgee-server/internal/idgen"
	"gorm.io/gorm"
)

var (
	ErrHelperJobInvalidInput          = errors.New("helper job: invalid input")
	ErrHelperJobUnknownType           = errors.New("helper job: unknown type")
	ErrHelperJobTypeNotEnabled        = errors.New("helper job: type not enabled")
	ErrHelperJobSchemaInvalid         = errors.New("helper job: schema invalid")
	ErrHelperJobForbiddenField        = errors.New("helper job: forbidden field")
	ErrHelperJobEnrollmentNotFound    = errors.New("helper job: enrollment not found")
	ErrHelperJobForbidden             = errors.New("helper job: forbidden")
	ErrHelperJobWrongOwner            = errors.New("helper job: wrong owner")
	ErrHelperJobWrongOrg              = errors.New("helper job: wrong org")
	ErrHelperJobEnrollmentInactive    = errors.New("helper job: enrollment inactive")
	ErrHelperJobEnrollmentUnclaimed   = errors.New("helper job: enrollment unclaimed")
	ErrHelperJobEnrollmentRevoked     = errors.New("helper job: enrollment revoked")
	ErrHelperJobEnrollmentUninstalled = errors.New("helper job: enrollment uninstalled")
	ErrHelperJobStaleEnrollment       = errors.New("helper job: stale enrollment")
	ErrHelperJobDelegationDenied      = errors.New("helper job: delegation denied")
	ErrHelperJobManifestRequired      = errors.New("helper job: manifest required")
	ErrHelperJobIdempotencyConflict   = errors.New("helper job: idempotency conflict")
	ErrHelperJobExpired               = errors.New("helper job: expired")
)

const (
	HelperJobTypeOpenClawConfigureAgent = "openclaw.configure_agent"
	HelperJobStatusQueued               = "queued"
	HelperJobStatusExpired              = "expired"
	HelperJobDefaultTTL                 = 5 * time.Minute
	HelperJobFreshnessWindow            = 5 * time.Minute
)

type EnqueueHelperJobInput struct {
	OwnerUserID    string
	OrgID          string
	EnrollmentID   string
	JobType        string
	SchemaVersion  int
	PayloadJSON    string
	IdempotencyKey string
}

type helperJobSpec struct {
	Category string
	Enabled  bool
	Manifest bool
}

var helperJobTaxonomy = map[string]helperJobSpec{
	"openclaw.configure_agent":           {Category: "openclaw_config", Enabled: true},
	"openclaw.install_from_manifest":     {Category: "openclaw_lifecycle", Manifest: true},
	"borgee_plugin.configure_connection": {Category: "openclaw_config"},
	"service.lifecycle":                  {Category: "openclaw_lifecycle"},
	"state.write":                        {Category: "openclaw_config"},
	"status.collect":                     {Category: "status_collect"},
	"delegation.revoke":                  {Category: "helper_lifecycle"},
	"helper.uninstall":                   {Category: "helper_lifecycle"},
}

type openClawConfigurePayload struct {
	AgentID   string `json:"agent_id"`
	ChannelID string `json:"channel_id,omitempty"`
}

type openClawEffectivePayload struct {
	AgentID             string `json:"agent_id"`
	ChannelID           string `json:"channel_id,omitempty"`
	ConfigSchemaVersion int64  `json:"config_schema_version"`
	ConfigHash          string `json:"config_hash"`
}

type helperAgentConfigRow struct {
	AgentID       string `gorm:"column:agent_id"`
	SchemaVersion int64  `gorm:"column:schema_version"`
	Blob          string `gorm:"column:blob"`
}

// EnqueueHelperJobForUser creates or converges one active, server-authorized
// Helper job. It does not expose Helper poll, lease, result, ack, or execution
// behavior.
func (s *Store) EnqueueHelperJobForUser(input EnqueueHelperJobInput, now time.Time) (*HelperJob, bool, error) {
	input.OwnerUserID = strings.TrimSpace(input.OwnerUserID)
	input.OrgID = strings.TrimSpace(input.OrgID)
	input.EnrollmentID = strings.TrimSpace(input.EnrollmentID)
	input.JobType = strings.TrimSpace(input.JobType)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if input.OwnerUserID == "" || input.OrgID == "" || input.EnrollmentID == "" || input.JobType == "" || input.SchemaVersion == 0 {
		return nil, false, ErrHelperJobInvalidInput
	}
	if len(input.IdempotencyKey) > 128 {
		return nil, false, ErrHelperJobInvalidInput
	}
	spec, ok := helperJobTaxonomy[input.JobType]
	if !ok {
		return nil, false, ErrHelperJobUnknownType
	}
	if input.SchemaVersion != 1 {
		return nil, false, ErrHelperJobSchemaInvalid
	}
	if spec.Manifest {
		return nil, false, ErrHelperJobManifestRequired
	}
	if !spec.Enabled {
		return nil, false, ErrHelperJobTypeNotEnabled
	}

	var out HelperJob
	created := false
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := expireActiveHelperJobs(tx, now); err != nil {
			return err
		}
		if !helperJobOwnerIsHumanMember(tx, input.OwnerUserID, input.OrgID) {
			return ErrHelperJobForbidden
		}

		var enrollment HelperEnrollment
		if err := tx.Where("id = ?", input.EnrollmentID).First(&enrollment).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrHelperJobEnrollmentNotFound
			}
			return err
		}
		if enrollment.OwnerUserID != input.OwnerUserID {
			return errors.Join(ErrHelperJobForbidden, ErrHelperJobWrongOwner)
		}
		if enrollment.OrgID != input.OrgID {
			return errors.Join(ErrHelperJobForbidden, ErrHelperJobWrongOrg)
		}
		if err := validateHelperEnrollmentForJob(&enrollment, spec.Category, now); err != nil {
			return err
		}

		effectivePayload, payloadHash, manifestDigest, err := s.effectiveHelperJobPayload(tx, input, now)
		if err != nil {
			return err
		}
		idempotencyScope := helperJobScope(input, payloadHash, manifestDigest)
		if input.IdempotencyKey != "" {
			conflict, err := activeHelperJobWithSameClientKey(tx, input, idempotencyScope)
			if err != nil {
				return err
			}
			if conflict {
				return ErrHelperJobIdempotencyConflict
			}
		}

		var existing HelperJob
		if err := tx.Where("active_idempotency_scope = ?", idempotencyScope).First(&existing).Error; err == nil {
			out = existing
			created = false
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		createdAt := now.UnixMilli()
		expiresAt := now.Add(HelperJobDefaultTTL).UnixMilli()
		activeScope := idempotencyScope
		var idemKey *string
		if input.IdempotencyKey != "" {
			key := input.IdempotencyKey
			idemKey = &key
		}
		row := HelperJob{
			ID:                     idgen.NewID(),
			OwnerUserID:            input.OwnerUserID,
			OrgID:                  input.OrgID,
			EnrollmentID:           enrollment.ID,
			HelperDeviceID:         enrollment.HelperDeviceID,
			JobType:                input.JobType,
			Category:               spec.Category,
			SchemaVersion:          input.SchemaVersion,
			PayloadJSON:            string(effectivePayload),
			PayloadHash:            payloadHash,
			ManifestDigest:         manifestDigest,
			IdempotencyKey:         idemKey,
			IdempotencyScope:       idempotencyScope,
			ActiveIdempotencyScope: &activeScope,
			Status:                 HelperJobStatusQueued,
			CreatedAt:              createdAt,
			UpdatedAt:              createdAt,
			ExpiresAt:              expiresAt,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		out = row
		created = true
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return &out, created, nil
}

func validateHelperEnrollmentForJob(row *HelperEnrollment, category string, now time.Time) error {
	if row.OwnerUserID == "" || row.OrgID == "" || row.Status == "pending" || row.ClaimedAt == nil || row.HelperDeviceID == nil || row.PersistentCredentialDigest == nil {
		return ErrHelperJobEnrollmentUnclaimed
	}
	if row.Status == "revoked" || row.RevokedAt != nil {
		return errors.Join(ErrHelperJobEnrollmentInactive, ErrHelperJobEnrollmentRevoked)
	}
	if row.Status == "uninstalled" || row.UninstalledAt != nil {
		return errors.Join(ErrHelperJobEnrollmentInactive, ErrHelperJobEnrollmentUninstalled)
	}
	if row.LastSeenAt == nil {
		return errors.Join(ErrHelperJobEnrollmentInactive, ErrHelperJobStaleEnrollment)
	}
	lastSeen := time.UnixMilli(*row.LastSeenAt)
	if lastSeen.After(now) || now.Sub(lastSeen) > HelperJobFreshnessWindow {
		return errors.Join(ErrHelperJobEnrollmentInactive, ErrHelperJobStaleEnrollment)
	}
	for _, allowed := range row.AllowedCategoryList() {
		if allowed == category {
			return nil
		}
	}
	return ErrHelperJobDelegationDenied
}

func helperJobOwnerIsHumanMember(tx *gorm.DB, ownerUserID, orgID string) bool {
	var count int64
	tx.Model(&User{}).
		Where("id = ? AND org_id = ? AND role = 'member' AND deleted_at IS NULL", ownerUserID, orgID).
		Count(&count)
	return count == 1
}

func (s *Store) effectiveHelperJobPayload(tx *gorm.DB, input EnqueueHelperJobInput, now time.Time) ([]byte, string, string, error) {
	switch input.JobType {
	case HelperJobTypeOpenClawConfigureAgent:
		payload, err := decodeOpenClawConfigurePayload(input.PayloadJSON)
		if err != nil {
			return nil, "", "", err
		}
		var agent User
		if err := tx.Where("id = ? AND role = 'agent' AND owner_id = ? AND org_id = ? AND deleted_at IS NULL", payload.AgentID, input.OwnerUserID, input.OrgID).First(&agent).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, "", "", ErrHelperJobForbidden
			}
			return nil, "", "", err
		}
		if payload.ChannelID != "" {
			var ch Channel
			if err := tx.Where("id = ? AND deleted_at IS NULL", payload.ChannelID).First(&ch).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil, "", "", ErrHelperJobForbidden
				}
				return nil, "", "", err
			}
			if ch.OrgID != input.OrgID || !s.CanAccessChannel(payload.ChannelID, input.OwnerUserID) || !s.CanAccessChannel(payload.ChannelID, agent.ID) {
				return nil, "", "", ErrHelperJobForbidden
			}
		}
		var cfg helperAgentConfigRow
		if err := tx.Raw(`SELECT agent_id, schema_version, blob FROM agent_configs WHERE agent_id = ?`, payload.AgentID).Scan(&cfg).Error; err != nil {
			return nil, "", "", err
		}
		if cfg.AgentID == "" {
			return nil, "", "", ErrHelperJobSchemaInvalid
		}
		canonicalConfig, err := canonicalJSON([]byte(cfg.Blob))
		if err != nil {
			return nil, "", "", ErrHelperJobSchemaInvalid
		}
		effective := openClawEffectivePayload{
			AgentID:             payload.AgentID,
			ChannelID:           payload.ChannelID,
			ConfigSchemaVersion: cfg.SchemaVersion,
			ConfigHash:          helperJobDigest(canonicalConfig),
		}
		b, err := json.Marshal(effective)
		if err != nil {
			return nil, "", "", err
		}
		return b, helperJobDigest(b), helperJobDigest([]byte("no-manifest")), nil
	default:
		return nil, "", "", ErrHelperJobUnknownType
	}
}

func decodeOpenClawConfigurePayload(raw string) (openClawConfigurePayload, error) {
	var pre map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &pre); err != nil || pre == nil {
		return openClawConfigurePayload{}, ErrHelperJobSchemaInvalid
	}
	for k := range pre {
		if helperJobForbiddenPayloadField(k) {
			return openClawConfigurePayload{}, ErrHelperJobForbiddenField
		}
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
	dec.DisallowUnknownFields()
	var payload openClawConfigurePayload
	if err := dec.Decode(&payload); err != nil {
		return openClawConfigurePayload{}, ErrHelperJobSchemaInvalid
	}
	payload.AgentID = strings.TrimSpace(payload.AgentID)
	payload.ChannelID = strings.TrimSpace(payload.ChannelID)
	if payload.AgentID == "" || len(payload.AgentID) > 255 || len(payload.ChannelID) > 255 {
		return openClawConfigurePayload{}, ErrHelperJobSchemaInvalid
	}
	return payload, nil
}

func helperJobForbiddenPayloadField(k string) bool {
	switch strings.ToLower(strings.TrimSpace(k)) {
	case "shell", "argv", "command", "raw_command", "executable_path", "script", "service_unit", "path", "domain", "url", "credential", "credentials", "token", "env", "environment", "owner_user_id", "org_id", "device_id", "helper_device_id", "category", "agent_config_id", "config_hash", "config_version", "schema_hash", "ttl", "expires_at", "deadline", "lease_expires_at":
		return true
	default:
		return false
	}
}

func expireActiveHelperJobs(tx *gorm.DB, now time.Time) error {
	ts := now.UnixMilli()
	return tx.Model(&HelperJob{}).
		Where("active_idempotency_scope IS NOT NULL AND status IN ? AND expires_at <= ?", []string{"queued", "leased", "running"}, ts).
		Updates(map[string]any{
			"status":                   HelperJobStatusExpired,
			"failure_code":             "ttl_expired",
			"updated_at":               ts,
			"completed_at":             ts,
			"active_idempotency_scope": nil,
		}).Error
}

func activeHelperJobWithSameClientKey(tx *gorm.DB, input EnqueueHelperJobInput, scope string) (bool, error) {
	var count int64
	err := tx.Model(&HelperJob{}).
		Where("owner_user_id = ? AND org_id = ? AND enrollment_id = ? AND job_type = ? AND schema_version = ? AND idempotency_key = ? AND active_idempotency_scope IS NOT NULL AND idempotency_scope <> ?",
			input.OwnerUserID, input.OrgID, input.EnrollmentID, input.JobType, input.SchemaVersion, input.IdempotencyKey, scope).
		Count(&count).Error
	return count > 0, err
}

func helperJobScope(input EnqueueHelperJobInput, payloadHash, manifestDigest string) string {
	parts := []string{input.OwnerUserID, input.OrgID, input.EnrollmentID, input.JobType, fmt.Sprint(input.SchemaVersion), payloadHash, manifestDigest, input.IdempotencyKey}
	return helperJobDigest([]byte(strings.Join(parts, "\x00")))
}

func helperJobDigest(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func canonicalJSON(raw []byte) ([]byte, error) {
	var v any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return json.Marshal(v)
}
