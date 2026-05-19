package store

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
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
	ErrHelperJobUnauthorized          = errors.New("helper job: unauthorized")
	ErrHelperJobStaleCredential       = errors.New("helper job: stale credential")
	ErrHelperJobDeviceMismatch        = errors.New("helper job: device mismatch")
	ErrHelperJobNoWork                = errors.New("helper job: no work")
	ErrHelperJobLeaseLost             = errors.New("helper job: lease lost")
	ErrHelperJobTerminalConflict      = errors.New("helper job: terminal conflict")
	ErrHelperJobNotFound              = errors.New("helper job: not found")
)

const (
	HelperJobTypeOpenClawConfigureAgent      = "openclaw.configure_agent"
	HelperJobTypeOpenClawInstallFromManifest = "openclaw.install_from_manifest"
	HelperJobTypePluginConfigureConnection   = "borgee_plugin.configure_connection"
	HelperJobTypeServiceLifecycle            = "service.lifecycle"
	HelperJobTypeHelperUninstall             = "helper.uninstall"
	HelperJobStatusQueued                    = "queued"
	HelperJobStatusLeased                    = "leased"
	HelperJobStatusRunning                   = "running"
	HelperJobStatusSucceeded                 = "succeeded"
	HelperJobStatusFailed                    = "failed"
	HelperJobStatusCancelled                 = "cancelled"
	HelperJobStatusExpired                   = "expired"
	HelperJobDefaultTTL                      = 5 * time.Minute
	HelperJobFreshnessWindow                 = 5 * time.Minute
	HelperJobDefaultLeaseDuration            = time.Minute
	HelperJobDefaultRetryAfterNoWork         = 5 * time.Second
	HelperJobPollLeased                      = "leased"
	HelperJobPollNoWork                      = "no_work"
)

const (
	helperJobOpenClawManifestSeed      = "borgee-helper-openclaw-runtime-policy-manifest-v1"
	helperJobOpenClawPluginArtifactID  = "openclaw-plugin"
	helperJobOpenClawInstallPathID     = "openclaw_install"
	helperJobOpenClawAgentConfigPathID = "openclaw_agent_config"
	helperJobBorgeePluginConfigPathID  = "borgee_plugin_config"
	helperJobOpenClawServiceID         = "openclaw-user"
	helperJobOpenClawPluginOrigin      = "https://cdn.borgee.io"
	helperJobOpenClawPluginInstallPlan = "openclaw-plugin-v1"
	helperJobOpenClawRuntimeIdentifier = "openclaw"

	// helper.uninstall manifest binding ids — the helper's own state dirs +
	// system service unit. Server publishes these so the helper-side jobpolicy
	// authority checks (validateManifestRequirements / validatePaths /
	// validateServices) accept the leased uninstall job. The actual filesystem
	// paths live on the helper side (executors/uninstall) — these IDs are
	// purely the manifest authority handle.
	helperJobHelperUninstallStatePathID   = "helper_state"
	helperJobHelperUninstallRuntimePathID = "helper_runtime"
	helperJobHelperUninstallServiceID     = "borgee-helper-service"
)

type PollHelperJobInput struct {
	EnrollmentID      string
	HelperCredential  string
	HelperDeviceID    string
	LeaseDuration     time.Duration
	RetryAfterNoWork  time.Duration
	MaxActiveLeases   int
	AllowedCategories []string
}

type AckHelperJobInput struct {
	EnrollmentID     string
	JobID            string
	HelperCredential string
	HelperDeviceID   string
	LeaseToken       string
	AckStatus        string
}

type CompleteHelperJobInput struct {
	EnrollmentID       string
	JobID              string
	HelperCredential   string
	HelperDeviceID     string
	LeaseToken         string
	Status             string
	FailureCode        string
	FailureMessage     string
	ResultSummaryJSON  string
	MaxFailureMessage  int
	MaxResultSummaries int
}

type HelperJobLease struct {
	Status         string
	Job            *HelperJob
	LeaseToken     string
	LeaseExpiresAt int64
	Attempt        int
	RetryAfter     time.Duration
}

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
	"openclaw.configure_agent":           {Category: "openclaw_config", Enabled: true, Manifest: true},
	"openclaw.install_from_manifest":     {Category: "openclaw_lifecycle", Enabled: true, Manifest: true},
	"borgee_plugin.configure_connection": {Category: "openclaw_config", Enabled: true, Manifest: true},
	"service.lifecycle":                  {Category: "openclaw_lifecycle", Enabled: true, Manifest: true},
	"state.write":                        {Category: "openclaw_config"},
	"status.collect":                     {Category: "status_collect"},
	"delegation.revoke":                  {Category: "helper_lifecycle"},
	"helper.uninstall":                   {Category: "helper_lifecycle", Enabled: true, Manifest: true},
}

type openClawConfigurePayload struct {
	AgentID   string `json:"agent_id"`
	ChannelID string `json:"channel_id,omitempty"`
}

type openClawInstallPayload struct {
	Runtime string `json:"runtime"`
}

type borgeePluginConfigurePayload struct {
	AgentID   string `json:"agent_id"`
	ChannelID string `json:"channel_id"`
}

type serviceLifecyclePayload struct {
	Target    string `json:"target"`
	Operation string `json:"operation"`
}

type openClawEffectivePayload struct {
	AgentID             string `json:"agent_id"`
	ChannelID           string `json:"channel_id,omitempty"`
	ConfigSchemaVersion int64  `json:"config_schema_version"`
	ConfigHash          string `json:"config_hash"`
}

type openClawInstallEffectivePayload struct {
	InstallPlanID string `json:"install_plan_id"`
}

type borgeePluginEffectivePayload struct {
	ConnectionID string `json:"connection_id"`
	AgentID      string `json:"agent_id"`
	ChannelID    string `json:"channel_id"`
}

type serviceLifecycleEffectivePayload struct {
	Operation string `json:"operation"`
}

type helperUninstallPayload struct {
	Scope         string `json:"scope"`
	PreserveState bool   `json:"preserve_state,omitempty"`
}

type helperUninstallEffectivePayload struct {
	Scope         string `json:"scope"`
	PreserveState bool   `json:"preserve_state,omitempty"`
}

type helperJobManifestBinding struct {
	ManifestDigest string   `json:"manifest_digest"`
	ArtifactIDs    []string `json:"artifact_ids,omitempty"`
	PathIDs        []string `json:"path_ids,omitempty"`
	Domains        []string `json:"domains,omitempty"`
	ServiceIDs     []string `json:"service_ids,omitempty"`
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

		effectivePayload, payloadHash, manifestDigest, manifestBindingJSON, err := s.effectiveHelperJobPayload(tx, input, now)
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
			ManifestBindingJSON:    manifestBindingJSON,
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

func (s *Store) PollAndLeaseHelperJobForHelper(input PollHelperJobInput, now time.Time) (*HelperJobLease, error) {
	input.EnrollmentID = strings.TrimSpace(input.EnrollmentID)
	input.HelperCredential = strings.TrimSpace(input.HelperCredential)
	input.HelperDeviceID = strings.TrimSpace(input.HelperDeviceID)
	if input.EnrollmentID == "" || input.HelperCredential == "" || input.HelperDeviceID == "" || len(input.HelperDeviceID) > 255 {
		return nil, ErrHelperJobInvalidInput
	}
	leaseDuration := input.LeaseDuration
	if leaseDuration <= 0 || leaseDuration > HelperJobDefaultTTL {
		leaseDuration = HelperJobDefaultLeaseDuration
	}
	retryAfter := input.RetryAfterNoWork
	if retryAfter <= 0 {
		retryAfter = HelperJobDefaultRetryAfterNoWork
	}
	maxActive := input.MaxActiveLeases
	if maxActive <= 0 {
		maxActive = 1
	}

	var out HelperJobLease
	var authErr error
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := expireActiveHelperJobs(tx, now); err != nil {
			return err
		}
		if err := expireLeaseLostHelperJobs(tx, now); err != nil {
			return err
		}
		enrollment, err := validateHelperJobRouteAuthority(tx, input.EnrollmentID, input.HelperCredential, input.HelperDeviceID)
		if err != nil {
			if errors.Is(err, ErrHelperJobStaleCredential) {
				if settleErr := settleActiveHelperJobsForEnrollment(tx, input.EnrollmentID, now, "stale_credential"); settleErr != nil {
					return settleErr
				}
			} else if errors.Is(err, ErrHelperJobEnrollmentRevoked) {
				if settleErr := settleHelperJobsForEnrollment(tx, input.EnrollmentID, now, "revoked"); settleErr != nil {
					return settleErr
				}
			} else if errors.Is(err, ErrHelperJobEnrollmentUninstalled) {
				if settleErr := settleHelperJobsForEnrollment(tx, input.EnrollmentID, now, "uninstalled"); settleErr != nil {
					return settleErr
				}
			}
			authErr = err
			return nil
		}
		var activeCount int64
		if err := tx.Model(&HelperJob{}).
			Where("enrollment_id = ? AND helper_device_id = ? AND status IN ? AND lease_expires_at > ? AND expires_at > ?", enrollment.ID, input.HelperDeviceID, []string{HelperJobStatusLeased, HelperJobStatusRunning}, now.UnixMilli(), now.UnixMilli()).
			Count(&activeCount).Error; err != nil {
			return err
		}
		if activeCount >= int64(maxActive) {
			out = HelperJobLease{Status: HelperJobPollNoWork, RetryAfter: retryAfter}
			return nil
		}

		var row HelperJob
		if err := tx.Where("enrollment_id = ? AND helper_device_id = ? AND status = ? AND expires_at > ? AND active_idempotency_scope IS NOT NULL", enrollment.ID, input.HelperDeviceID, HelperJobStatusQueued, now.UnixMilli()).
			Order("created_at ASC").First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				out = HelperJobLease{Status: HelperJobPollNoWork, RetryAfter: retryAfter}
				return nil
			}
			return err
		}
		leasedAt := now.UnixMilli()
		leaseExpiresAt := now.Add(leaseDuration).UnixMilli()
		res := tx.Model(&HelperJob{}).
			Where("id = ? AND status = ? AND active_idempotency_scope IS NOT NULL AND expires_at > ?", row.ID, HelperJobStatusQueued, now.UnixMilli()).
			Updates(map[string]any{
				"status":           HelperJobStatusLeased,
				"leased_at":        leasedAt,
				"lease_expires_at": leaseExpiresAt,
				"updated_at":       leasedAt,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			out = HelperJobLease{Status: HelperJobPollNoWork, RetryAfter: retryAfter}
			return nil
		}
		if err := tx.Where("id = ?", row.ID).First(&row).Error; err != nil {
			return err
		}
		leaseToken := helperJobLeaseToken(&row, enrollment)
		out = HelperJobLease{
			Status:         HelperJobPollLeased,
			Job:            helperJobLeaseProjection(&row),
			LeaseToken:     leaseToken,
			LeaseExpiresAt: leaseExpiresAt,
			Attempt:        1,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if authErr != nil {
		return nil, authErr
	}
	return &out, nil
}

func (s *Store) AckHelperJobForHelper(input AckHelperJobInput, now time.Time) (*HelperJob, error) {
	input.EnrollmentID = strings.TrimSpace(input.EnrollmentID)
	input.JobID = strings.TrimSpace(input.JobID)
	input.HelperCredential = strings.TrimSpace(input.HelperCredential)
	input.HelperDeviceID = strings.TrimSpace(input.HelperDeviceID)
	input.LeaseToken = strings.TrimSpace(input.LeaseToken)
	input.AckStatus = strings.TrimSpace(input.AckStatus)
	if input.EnrollmentID == "" || input.JobID == "" || input.HelperCredential == "" || input.HelperDeviceID == "" || input.LeaseToken == "" || input.AckStatus != "received" {
		return nil, ErrHelperJobInvalidInput
	}
	var out HelperJob
	var authErr error
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := expireActiveHelperJobs(tx, now); err != nil {
			return err
		}
		if err := expireLeaseLostHelperJobs(tx, now); err != nil {
			return err
		}
		enrollment, err := validateHelperJobRouteAuthority(tx, input.EnrollmentID, input.HelperCredential, input.HelperDeviceID)
		if err != nil {
			if errors.Is(err, ErrHelperJobStaleCredential) {
				if settleErr := settleActiveHelperJobsForEnrollment(tx, input.EnrollmentID, now, "stale_credential"); settleErr != nil {
					return settleErr
				}
			} else if errors.Is(err, ErrHelperJobEnrollmentRevoked) {
				if settleErr := settleHelperJobsForEnrollment(tx, input.EnrollmentID, now, "revoked"); settleErr != nil {
					return settleErr
				}
			} else if errors.Is(err, ErrHelperJobEnrollmentUninstalled) {
				if settleErr := settleHelperJobsForEnrollment(tx, input.EnrollmentID, now, "uninstalled"); settleErr != nil {
					return settleErr
				}
			}
			authErr = err
			return nil
		}
		row, err := loadHelperJobForRoute(tx, input.EnrollmentID, input.JobID)
		if err != nil {
			return err
		}
		if !helperJobLeaseTokenMatches(&row, enrollment, input.LeaseToken) {
			return ErrHelperJobLeaseLost
		}
		if row.Status == HelperJobStatusExpired && stringValue(row.FailureCode) == "lease_lost" {
			out = row
			return ErrHelperJobLeaseLost
		}
		if row.Status == HelperJobStatusRunning || helperJobIsTerminal(row.Status) {
			out = row
			return nil
		}
		if row.Status != HelperJobStatusLeased {
			return ErrHelperJobLeaseLost
		}
		if row.LeaseExpiresAt == nil || *row.LeaseExpiresAt <= now.UnixMilli() || row.ExpiresAt <= now.UnixMilli() {
			if err := settleHelperJob(tx, row.ID, now, HelperJobStatusExpired, "lease_lost", ""); err != nil {
				return err
			}
			return ErrHelperJobLeaseLost
		}
		res := tx.Model(&HelperJob{}).
			Where("id = ? AND status = ?", row.ID, HelperJobStatusLeased).
			Updates(map[string]any{"status": HelperJobStatusRunning, "updated_at": now.UnixMilli()})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrHelperJobLeaseLost
		}
		if err := tx.Where("id = ?", row.ID).First(&out).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if authErr != nil {
		return nil, authErr
	}
	return &out, nil
}

func (s *Store) CompleteHelperJobForHelper(input CompleteHelperJobInput, now time.Time) (*HelperJob, error) {
	input.EnrollmentID = strings.TrimSpace(input.EnrollmentID)
	input.JobID = strings.TrimSpace(input.JobID)
	input.HelperCredential = strings.TrimSpace(input.HelperCredential)
	input.HelperDeviceID = strings.TrimSpace(input.HelperDeviceID)
	input.LeaseToken = strings.TrimSpace(input.LeaseToken)
	input.Status = strings.TrimSpace(input.Status)
	input.FailureCode = strings.TrimSpace(input.FailureCode)
	input.FailureMessage = strings.TrimSpace(input.FailureMessage)
	if input.EnrollmentID == "" || input.JobID == "" || input.HelperCredential == "" || input.HelperDeviceID == "" || input.LeaseToken == "" {
		return nil, ErrHelperJobInvalidInput
	}
	failureMessage, resultSummary, err := validateHelperJobTerminalInput(input)
	if err != nil {
		return nil, err
	}
	var out HelperJob
	var authErr error
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := expireActiveHelperJobs(tx, now); err != nil {
			return err
		}
		if err := expireLeaseLostHelperJobs(tx, now); err != nil {
			return err
		}
		enrollment, err := validateHelperJobRouteAuthority(tx, input.EnrollmentID, input.HelperCredential, input.HelperDeviceID)
		if err != nil {
			if errors.Is(err, ErrHelperJobStaleCredential) {
				if settleErr := settleActiveHelperJobsForEnrollment(tx, input.EnrollmentID, now, "stale_credential"); settleErr != nil {
					return settleErr
				}
			} else if errors.Is(err, ErrHelperJobEnrollmentRevoked) {
				if settleErr := settleHelperJobsForEnrollment(tx, input.EnrollmentID, now, "revoked"); settleErr != nil {
					return settleErr
				}
			} else if errors.Is(err, ErrHelperJobEnrollmentUninstalled) {
				if settleErr := settleHelperJobsForEnrollment(tx, input.EnrollmentID, now, "uninstalled"); settleErr != nil {
					return settleErr
				}
			}
			authErr = err
			return nil
		}
		row, err := loadHelperJobForRoute(tx, input.EnrollmentID, input.JobID)
		if err != nil {
			return err
		}
		if !helperJobLeaseTokenMatches(&row, enrollment, input.LeaseToken) {
			return ErrHelperJobLeaseLost
		}
		if row.Status == HelperJobStatusExpired && stringValue(row.FailureCode) == "lease_lost" {
			out = row
			return ErrHelperJobLeaseLost
		}
		if helperJobIsTerminal(row.Status) {
			if helperJobTerminalMatches(&row, input.Status, input.FailureCode, failureMessage, resultSummary) {
				out = row
				return nil
			}
			return ErrHelperJobTerminalConflict
		}
		if row.Status != HelperJobStatusLeased && row.Status != HelperJobStatusRunning {
			return ErrHelperJobLeaseLost
		}
		if row.LeaseExpiresAt == nil || *row.LeaseExpiresAt <= now.UnixMilli() || row.ExpiresAt <= now.UnixMilli() {
			if err := settleHelperJob(tx, row.ID, now, HelperJobStatusExpired, "lease_lost", ""); err != nil {
				return err
			}
			return ErrHelperJobLeaseLost
		}
		updates := map[string]any{
			"status":                   input.Status,
			"failure_code":             nil,
			"failure_message":          nil,
			"result_summary_json":      nil,
			"completed_at":             now.UnixMilli(),
			"updated_at":               now.UnixMilli(),
			"active_idempotency_scope": nil,
		}
		if input.FailureCode != "" {
			updates["failure_code"] = input.FailureCode
		}
		if failureMessage != "" {
			updates["failure_message"] = failureMessage
		}
		if resultSummary != "" {
			updates["result_summary_json"] = resultSummary
		}
		res := tx.Model(&HelperJob{}).
			Where("id = ? AND status IN ?", row.ID, []string{HelperJobStatusLeased, HelperJobStatusRunning}).
			Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrHelperJobTerminalConflict
		}
		if err := tx.Where("id = ?", row.ID).First(&out).Error; err != nil {
			return err
		}
		// #998 — On terminal succeeded for `helper.uninstall`, flip the
		// enrollment status to `uninstalled` in the same transaction so the
		// server-recorded enrollment state matches the helper's local
		// teardown. Operator does not need to also call the dedicated
		// /uninstall endpoint — the executor reaching terminal success IS the
		// server-side signal. Non-succeeded terminals (failed / cancelled /
		// expired) leave the enrollment alone so an operator can retry.
		if row.JobType == HelperJobTypeHelperUninstall && input.Status == HelperJobStatusSucceeded {
			if err := markHelperEnrollmentUninstalledInTx(tx, row.EnrollmentID, now); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if authErr != nil {
		return nil, authErr
	}
	return &out, nil
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

func (s *Store) effectiveHelperJobPayload(tx *gorm.DB, input EnqueueHelperJobInput, now time.Time) ([]byte, string, string, *string, error) {
	switch input.JobType {
	case HelperJobTypeOpenClawConfigureAgent:
		payload, err := decodeOpenClawConfigurePayload(input.PayloadJSON)
		if err != nil {
			return nil, "", "", nil, err
		}
		var agent User
		if err := tx.Where("id = ? AND role = 'agent' AND owner_id = ? AND org_id = ? AND deleted_at IS NULL", payload.AgentID, input.OwnerUserID, input.OrgID).First(&agent).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, "", "", nil, ErrHelperJobForbidden
			}
			return nil, "", "", nil, err
		}
		if payload.ChannelID != "" {
			var ch Channel
			if err := tx.Where("id = ? AND deleted_at IS NULL", payload.ChannelID).First(&ch).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil, "", "", nil, ErrHelperJobForbidden
				}
				return nil, "", "", nil, err
			}
			if ch.OrgID != input.OrgID || !s.CanAccessChannel(payload.ChannelID, input.OwnerUserID) || !s.CanAccessChannel(payload.ChannelID, agent.ID) {
				return nil, "", "", nil, ErrHelperJobForbidden
			}
		}
		var cfg helperAgentConfigRow
		if err := tx.Raw(`SELECT agent_id, schema_version, blob FROM agent_configs WHERE agent_id = ?`, payload.AgentID).Scan(&cfg).Error; err != nil {
			return nil, "", "", nil, err
		}
		if cfg.AgentID == "" {
			return nil, "", "", nil, ErrHelperJobSchemaInvalid
		}
		canonicalConfig, err := canonicalJSON([]byte(cfg.Blob))
		if err != nil {
			return nil, "", "", nil, ErrHelperJobSchemaInvalid
		}
		effective := openClawEffectivePayload{
			AgentID:             payload.AgentID,
			ChannelID:           payload.ChannelID,
			ConfigSchemaVersion: cfg.SchemaVersion,
			ConfigHash:          helperJobDigest(canonicalConfig),
		}
		b, err := json.Marshal(effective)
		if err != nil {
			return nil, "", "", nil, err
		}
		manifestDigest, bindingJSON, err := openClawManifestBindingForJob(input.JobType)
		if err != nil {
			return nil, "", "", nil, err
		}
		return b, helperJobDigest(b), manifestDigest, bindingJSON, nil
	case HelperJobTypeOpenClawInstallFromManifest:
		payload, err := decodeOpenClawInstallPayload(input.PayloadJSON)
		if err != nil {
			return nil, "", "", nil, err
		}
		if payload.Runtime != helperJobOpenClawRuntimeIdentifier {
			return nil, "", "", nil, ErrHelperJobSchemaInvalid
		}
		effective := openClawInstallEffectivePayload{InstallPlanID: helperJobOpenClawPluginInstallPlan}
		b, err := json.Marshal(effective)
		if err != nil {
			return nil, "", "", nil, err
		}
		manifestDigest, bindingJSON, err := openClawManifestBindingForJob(input.JobType)
		if err != nil {
			return nil, "", "", nil, err
		}
		return b, helperJobDigest(b), manifestDigest, bindingJSON, nil
	case HelperJobTypePluginConfigureConnection:
		payload, err := decodeBorgeePluginConfigurePayload(input.PayloadJSON)
		if err != nil {
			return nil, "", "", nil, err
		}
		var agent User
		if err := tx.Where("id = ? AND role = 'agent' AND owner_id = ? AND org_id = ? AND deleted_at IS NULL", payload.AgentID, input.OwnerUserID, input.OrgID).First(&agent).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, "", "", nil, ErrHelperJobForbidden
			}
			return nil, "", "", nil, err
		}
		var ch Channel
		if err := tx.Where("id = ? AND deleted_at IS NULL", payload.ChannelID).First(&ch).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, "", "", nil, ErrHelperJobForbidden
			}
			return nil, "", "", nil, err
		}
		if ch.OrgID != input.OrgID || ch.Type != "channel" || !s.CanAccessChannel(payload.ChannelID, input.OwnerUserID) || !s.CanAccessChannel(payload.ChannelID, agent.ID) {
			return nil, "", "", nil, ErrHelperJobForbidden
		}
		effective := borgeePluginEffectivePayload{
			ConnectionID: serverOwnedBorgeePluginConnectionID(input.OrgID, payload.AgentID, payload.ChannelID),
			AgentID:      payload.AgentID,
			ChannelID:    payload.ChannelID,
		}
		b, err := json.Marshal(effective)
		if err != nil {
			return nil, "", "", nil, err
		}
		manifestDigest, bindingJSON, err := openClawManifestBindingForJob(input.JobType)
		if err != nil {
			return nil, "", "", nil, err
		}
		return b, helperJobDigest(b), manifestDigest, bindingJSON, nil
	case HelperJobTypeServiceLifecycle:
		payload, err := decodeServiceLifecyclePayload(input.PayloadJSON)
		if err != nil {
			return nil, "", "", nil, err
		}
		effective := serviceLifecycleEffectivePayload{Operation: payload.Operation}
		b, err := json.Marshal(effective)
		if err != nil {
			return nil, "", "", nil, err
		}
		manifestDigest, bindingJSON, err := openClawManifestBindingForJob(input.JobType)
		if err != nil {
			return nil, "", "", nil, err
		}
		return b, helperJobDigest(b), manifestDigest, bindingJSON, nil
	case HelperJobTypeHelperUninstall:
		payload, err := decodeHelperUninstallPayload(input.PayloadJSON)
		if err != nil {
			return nil, "", "", nil, err
		}
		effective := helperUninstallEffectivePayload{Scope: payload.Scope, PreserveState: payload.PreserveState}
		b, err := json.Marshal(effective)
		if err != nil {
			return nil, "", "", nil, err
		}
		manifestDigest, bindingJSON, err := openClawManifestBindingForJob(input.JobType)
		if err != nil {
			return nil, "", "", nil, err
		}
		return b, helperJobDigest(b), manifestDigest, bindingJSON, nil
	default:
		return nil, "", "", nil, ErrHelperJobUnknownType
	}
}

func serverOwnedBorgeePluginConnectionID(orgID, agentID, channelID string) string {
	digest := helperJobDigest([]byte(orgID + "|" + agentID + "|" + channelID))
	return "borgee-plugin:" + strings.TrimPrefix(digest, "sha256:")
}

func openClawManifestBindingForJob(jobType string) (string, *string, error) {
	manifestDigest := helperJobDigest([]byte(helperJobOpenClawManifestSeed))
	binding := helperJobManifestBinding{ManifestDigest: manifestDigest}
	switch jobType {
	case HelperJobTypeOpenClawConfigureAgent:
		binding.PathIDs = []string{helperJobOpenClawAgentConfigPathID}
	case HelperJobTypeOpenClawInstallFromManifest:
		binding.ArtifactIDs = []string{helperJobOpenClawPluginArtifactID}
		binding.PathIDs = []string{helperJobOpenClawInstallPathID, helperJobOpenClawAgentConfigPathID}
		binding.Domains = []string{helperJobOpenClawPluginOrigin}
	case HelperJobTypePluginConfigureConnection:
		binding.PathIDs = []string{helperJobBorgeePluginConfigPathID}
	case HelperJobTypeServiceLifecycle:
		binding.ServiceIDs = []string{helperJobOpenClawServiceID}
	case HelperJobTypeHelperUninstall:
		// helper.uninstall manifest authority: bind the helper's own state /
		// runtime paths + service unit so the helper-side jobpolicy gate
		// (validateManifestRequirements + validatePaths/validateServices)
		// accepts the leased uninstall job. No artifacts (the executor only
		// removes files), no domains (no network call), no path mode beyond
		// "write" (executor wipes — pathModeAllowsWrite covers that).
		binding.PathIDs = []string{helperJobHelperUninstallStatePathID, helperJobHelperUninstallRuntimePathID}
		binding.ServiceIDs = []string{helperJobHelperUninstallServiceID}
	default:
		return "", nil, ErrHelperJobUnknownType
	}
	raw, err := json.Marshal(binding)
	if err != nil {
		return "", nil, err
	}
	encoded := string(raw)
	return manifestDigest, &encoded, nil
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

func decodeOpenClawInstallPayload(raw string) (openClawInstallPayload, error) {
	var pre map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &pre); err != nil || pre == nil {
		return openClawInstallPayload{}, ErrHelperJobSchemaInvalid
	}
	for k := range pre {
		if helperJobForbiddenPayloadField(k) {
			return openClawInstallPayload{}, ErrHelperJobForbiddenField
		}
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
	dec.DisallowUnknownFields()
	var payload openClawInstallPayload
	if err := dec.Decode(&payload); err != nil {
		return openClawInstallPayload{}, ErrHelperJobSchemaInvalid
	}
	payload.Runtime = strings.TrimSpace(payload.Runtime)
	if payload.Runtime == "" || len(payload.Runtime) > 64 {
		return openClawInstallPayload{}, ErrHelperJobSchemaInvalid
	}
	return payload, nil
}

func decodeBorgeePluginConfigurePayload(raw string) (borgeePluginConfigurePayload, error) {
	var pre map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &pre); err != nil || pre == nil {
		return borgeePluginConfigurePayload{}, ErrHelperJobSchemaInvalid
	}
	for k := range pre {
		if helperJobForbiddenPayloadField(k) {
			return borgeePluginConfigurePayload{}, ErrHelperJobForbiddenField
		}
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
	dec.DisallowUnknownFields()
	var payload borgeePluginConfigurePayload
	if err := dec.Decode(&payload); err != nil {
		return borgeePluginConfigurePayload{}, ErrHelperJobSchemaInvalid
	}
	payload.AgentID = strings.TrimSpace(payload.AgentID)
	payload.ChannelID = strings.TrimSpace(payload.ChannelID)
	if payload.AgentID == "" || payload.ChannelID == "" || len(payload.AgentID) > 255 || len(payload.ChannelID) > 255 {
		return borgeePluginConfigurePayload{}, ErrHelperJobSchemaInvalid
	}
	return payload, nil
}

func decodeServiceLifecyclePayload(raw string) (serviceLifecyclePayload, error) {
	var pre map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &pre); err != nil || pre == nil {
		return serviceLifecyclePayload{}, ErrHelperJobSchemaInvalid
	}
	for k := range pre {
		if helperJobForbiddenPayloadField(k) {
			return serviceLifecyclePayload{}, ErrHelperJobForbiddenField
		}
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
	dec.DisallowUnknownFields()
	var payload serviceLifecyclePayload
	if err := dec.Decode(&payload); err != nil {
		return serviceLifecyclePayload{}, ErrHelperJobSchemaInvalid
	}
	payload.Target = strings.TrimSpace(payload.Target)
	payload.Operation = strings.TrimSpace(payload.Operation)
	if payload.Target != helperJobOpenClawRuntimeIdentifier || payload.Operation != "restart" {
		return serviceLifecyclePayload{}, ErrHelperJobSchemaInvalid
	}
	return payload, nil
}

// decodeHelperUninstallPayload validates the operator-supplied uninstall
// payload. Required fields: `scope: "helper"` (today the only supported
// scope — narrows future-proofing; an `agent` / `runtime` scope can be added
// without changing the wire shape). Optional: `preserve_state: bool` —
// when true, the helper-side executor skips wiping
// /var/lib/borgee-helper/{queue,status,audit-handoff} for forensic /
// post-mortem use. Unknown fields and the standard forbidden-field set
// (shell/url/credential/etc.) are rejected before reaching the helper.
func decodeHelperUninstallPayload(raw string) (helperUninstallPayload, error) {
	var pre map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &pre); err != nil || pre == nil {
		return helperUninstallPayload{}, ErrHelperJobSchemaInvalid
	}
	for k := range pre {
		if helperJobForbiddenPayloadField(k) {
			return helperUninstallPayload{}, ErrHelperJobForbiddenField
		}
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
	dec.DisallowUnknownFields()
	var payload helperUninstallPayload
	if err := dec.Decode(&payload); err != nil {
		return helperUninstallPayload{}, ErrHelperJobSchemaInvalid
	}
	payload.Scope = strings.TrimSpace(payload.Scope)
	if payload.Scope != "helper" {
		return helperUninstallPayload{}, ErrHelperJobSchemaInvalid
	}
	return payload, nil
}

func helperJobForbiddenPayloadField(k string) bool {
	switch strings.ToLower(strings.TrimSpace(k)) {
	case "shell", "argv", "command", "raw_command", "executable_path", "script", "service_unit", "path", "paths", "path_id", "path_ids", "domain", "domains", "domain_id", "domain_ids", "url", "base_url", "credential", "credentials", "token", "api_key", "bot_user_id", "account_id", "env", "environment", "owner_user_id", "org_id", "device_id", "helper_device_id", "category", "agent_config_id", "config_hash", "config_version", "schema_hash", "connection_id", "manifest_id", "manifest_digest", "manifest_binding", "manifest_binding_json", "artifact", "artifact_id", "artifact_ids", "service_id", "service_ids", "install_plan_id", "ttl", "expires_at", "deadline", "lease_expires_at":
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

func expireLeaseLostHelperJobs(tx *gorm.DB, now time.Time) error {
	ts := now.UnixMilli()
	return tx.Model(&HelperJob{}).
		Where("active_idempotency_scope IS NOT NULL AND status IN ? AND lease_expires_at IS NOT NULL AND lease_expires_at <= ?", []string{HelperJobStatusLeased, HelperJobStatusRunning}, ts).
		Updates(map[string]any{
			"status":                   HelperJobStatusExpired,
			"failure_code":             "lease_lost",
			"updated_at":               ts,
			"completed_at":             ts,
			"active_idempotency_scope": nil,
		}).Error
}

func validateHelperJobRouteAuthority(tx *gorm.DB, enrollmentID, credential, helperDeviceID string) (*HelperEnrollment, error) {
	var row HelperEnrollment
	if err := tx.Where("id = ?", enrollmentID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrHelperJobEnrollmentNotFound
		}
		return nil, err
	}
	if row.OwnerUserID == "" || row.OrgID == "" || row.Status == "pending" || row.ClaimedAt == nil || row.PersistentCredentialDigest == nil || row.HelperDeviceID == nil {
		return nil, ErrHelperJobEnrollmentUnclaimed
	}
	if row.Status == "revoked" || row.RevokedAt != nil {
		return nil, errors.Join(ErrHelperJobEnrollmentInactive, ErrHelperJobEnrollmentRevoked)
	}
	if row.Status == "uninstalled" || row.UninstalledAt != nil {
		return nil, errors.Join(ErrHelperJobEnrollmentInactive, ErrHelperJobEnrollmentUninstalled)
	}
	if !constantTimeDigestMatch(*row.PersistentCredentialDigest, credential) {
		if row.CredentialRotatedAt != nil {
			return nil, ErrHelperJobStaleCredential
		}
		return nil, ErrHelperJobUnauthorized
	}
	if *row.HelperDeviceID != helperDeviceID {
		return nil, ErrHelperJobDeviceMismatch
	}
	return &row, nil
}

func loadHelperJobForRoute(tx *gorm.DB, enrollmentID, jobID string) (HelperJob, error) {
	var row HelperJob
	if err := tx.Where("id = ? AND enrollment_id = ?", jobID, enrollmentID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return HelperJob{}, ErrHelperJobNotFound
		}
		return HelperJob{}, err
	}
	return row, nil
}

func helperJobLeaseProjection(row *HelperJob) *HelperJob {
	if row == nil {
		return nil
	}
	copy := *row
	copy.OwnerUserID = ""
	copy.OrgID = ""
	return &copy
}

func helperJobLeaseToken(row *HelperJob, enrollment *HelperEnrollment) string {
	if row == nil || enrollment == nil || enrollment.PersistentCredentialDigest == nil || row.LeasedAt == nil || row.LeaseExpiresAt == nil {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(*enrollment.PersistentCredentialDigest))
	_, _ = mac.Write([]byte(strings.Join([]string{
		row.ID,
		row.EnrollmentID,
		stringValue(row.HelperDeviceID),
		fmt.Sprint(*row.LeasedAt),
		fmt.Sprint(*row.LeaseExpiresAt),
	}, "\x00")))
	return "v1:" + hex.EncodeToString(mac.Sum(nil))
}

func helperJobLeaseTokenMatches(row *HelperJob, enrollment *HelperEnrollment, token string) bool {
	want := helperJobLeaseToken(row, enrollment)
	if want == "" || token == "" {
		return false
	}
	return hmac.Equal([]byte(want), []byte(token))
}

func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func helperJobIsTerminal(status string) bool {
	switch status {
	case HelperJobStatusSucceeded, HelperJobStatusFailed, HelperJobStatusCancelled, HelperJobStatusExpired:
		return true
	default:
		return false
	}
}

func settleHelperJobsForEnrollment(tx *gorm.DB, enrollmentID string, now time.Time, reason string) error {
	status := HelperJobStatusCancelled
	if reason == "ttl_expired" || reason == "lease_lost" {
		status = HelperJobStatusExpired
	}
	return tx.Model(&HelperJob{}).
		Where("enrollment_id = ? AND active_idempotency_scope IS NOT NULL AND status IN ?", enrollmentID, []string{HelperJobStatusQueued, HelperJobStatusLeased, HelperJobStatusRunning}).
		Updates(map[string]any{
			"status":                   status,
			"failure_code":             reason,
			"completed_at":             now.UnixMilli(),
			"updated_at":               now.UnixMilli(),
			"active_idempotency_scope": nil,
		}).Error
}

func settleActiveHelperJobsForEnrollment(tx *gorm.DB, enrollmentID string, now time.Time, reason string) error {
	return tx.Model(&HelperJob{}).
		Where("enrollment_id = ? AND active_idempotency_scope IS NOT NULL AND status IN ?", enrollmentID, []string{HelperJobStatusLeased, HelperJobStatusRunning}).
		Updates(map[string]any{
			"status":                   HelperJobStatusCancelled,
			"failure_code":             reason,
			"completed_at":             now.UnixMilli(),
			"updated_at":               now.UnixMilli(),
			"active_idempotency_scope": nil,
		}).Error
}

func settleHelperJob(tx *gorm.DB, jobID string, now time.Time, status, failureCode, failureMessage string) error {
	updates := map[string]any{
		"status":                   status,
		"failure_code":             failureCode,
		"completed_at":             now.UnixMilli(),
		"updated_at":               now.UnixMilli(),
		"active_idempotency_scope": nil,
	}
	if failureMessage != "" {
		updates["failure_message"] = failureMessage
	}
	return tx.Model(&HelperJob{}).Where("id = ?", jobID).Updates(updates).Error
}

// markHelperEnrollmentUninstalledInTx flips a helper enrollment to
// `uninstalled` from inside an existing transaction (used by
// CompleteHelperJobForHelper when a `helper.uninstall` job terminates
// `succeeded`). The credential / device-id authority check has already
// happened upstream via validateHelperJobRouteAuthority, so we do not
// re-validate them here — gating the UPDATE on `revoked_at IS NULL AND
// uninstalled_at IS NULL` is sufficient idempotency / race protection.
// Already-uninstalled rows are a no-op (returns nil); the helper.uninstall
// terminal-conflict path also lets the second uninstall resolve gracefully
// instead of looping.
func markHelperEnrollmentUninstalledInTx(tx *gorm.DB, enrollmentID string, now time.Time) error {
	ts := now.UnixMilli()
	res := tx.Model(&HelperEnrollment{}).
		Where("id = ? AND revoked_at IS NULL AND uninstalled_at IS NULL", enrollmentID).
		Updates(map[string]any{"status": "uninstalled", "uninstalled_at": ts, "updated_at": ts})
	return res.Error
}

func validateHelperJobTerminalInput(input CompleteHelperJobInput) (string, string, error) {
	if !validHelperJobTerminalStatus(input.Status) {
		return "", "", ErrHelperJobSchemaInvalid
	}
	if input.Status != HelperJobStatusSucceeded && !validHelperJobFailureCode(input.FailureCode) {
		return "", "", ErrHelperJobSchemaInvalid
	}
	if input.Status == HelperJobStatusSucceeded && input.FailureCode != "" {
		return "", "", ErrHelperJobSchemaInvalid
	}
	if input.Status == HelperJobStatusSucceeded && strings.TrimSpace(input.FailureMessage) != "" {
		return "", "", ErrHelperJobSchemaInvalid
	}
	maxMessage := input.MaxFailureMessage
	if maxMessage <= 0 || maxMessage > 1024 {
		maxMessage = 512
	}
	failureMessage := strings.Map(func(r rune) rune {
		if r < 32 && r != '\t' && r != '\n' {
			return -1
		}
		return r
	}, input.FailureMessage)
	if len(failureMessage) > maxMessage {
		return "", "", ErrHelperJobSchemaInvalid
	}
	failureMessage = redactHelperJobFailureMessage(failureMessage)
	resultSummary, err := normalizeHelperJobResultSummary(input.ResultSummaryJSON, input.MaxResultSummaries)
	if err != nil {
		return "", "", err
	}
	return failureMessage, resultSummary, nil
}

func validHelperJobTerminalStatus(status string) bool {
	switch status {
	case HelperJobStatusSucceeded, HelperJobStatusFailed, HelperJobStatusCancelled, HelperJobStatusExpired:
		return true
	default:
		return false
	}
}

func validHelperJobFailureCode(code string) bool {
	switch code {
	case "schema_invalid", "unknown_job_type", "policy_denied", "manifest_invalid", "artifact_invalid", "path_denied", "domain_denied", "service_denied", "revoked", "uninstalled", "stale_credential", "wrong_owner", "wrong_org", "ttl_expired", "lease_lost", "cancelled", "execution_failed":
		return true
	default:
		return false
	}
}

var helperJobFailureRedactors = []*regexp.Regexp{
	regexp.MustCompile(`(?i)authorization\s*:\s*bearer\s+[^\s]+`),
	regexp.MustCompile(`(?i)\b(token|credential|password|secret|api[_-]?key|authorization)\s*[:=]\s*[^\s]+`),
	regexp.MustCompile(`(?i)\benv\s*[:=]\s*[^\s]+`),
	regexp.MustCompile(`sk-[A-Za-z0-9_-]+`),
	regexp.MustCompile(`(?i)private\s+(file|message)\s+content`),
	regexp.MustCompile(`(/Users|/home)/[^\s]+`),
}

func redactHelperJobFailureMessage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	for _, re := range helperJobFailureRedactors {
		message = re.ReplaceAllString(message, "[redacted]")
	}
	return message
}

type helperJobResultSummary struct {
	AuditRefs []string `json:"audit_refs"`
	LogRefs   []string `json:"log_refs"`
}

func normalizeHelperJobResultSummary(raw string, maxRefs int) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if len(raw) > 4096 {
		return "", ErrHelperJobSchemaInvalid
	}
	if maxRefs <= 0 || maxRefs > 32 {
		maxRefs = 16
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &top); err != nil || top == nil {
		return "", ErrHelperJobSchemaInvalid
	}
	for key := range top {
		switch key {
		case "audit_refs", "log_refs":
		default:
			return "", ErrHelperJobForbiddenField
		}
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	var summary helperJobResultSummary
	if err := dec.Decode(&summary); err != nil {
		return "", ErrHelperJobSchemaInvalid
	}
	if len(summary.AuditRefs)+len(summary.LogRefs) > maxRefs {
		return "", ErrHelperJobSchemaInvalid
	}
	for _, ref := range append(append([]string{}, summary.AuditRefs...), summary.LogRefs...) {
		if strings.TrimSpace(ref) == "" || len(ref) > 128 || strings.ContainsAny(ref, "/\\\x00\n\r") {
			return "", ErrHelperJobSchemaInvalid
		}
	}
	b, err := json.Marshal(summary)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func helperJobTerminalMatches(row *HelperJob, status, failureCode, failureMessage, resultSummary string) bool {
	if row == nil || row.Status != status {
		return false
	}
	if stringValue(row.FailureCode) != failureCode {
		return false
	}
	if stringValue(row.FailureMessage) != failureMessage {
		return false
	}
	return stringValue(row.ResultSummaryJSON) == resultSummary
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
