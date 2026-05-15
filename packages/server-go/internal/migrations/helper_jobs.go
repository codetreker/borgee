package migrations

import "gorm.io/gorm"

// helperJobs is migration v=51 for the Helper typed job enqueue boundary. It
// creates only durable enqueue metadata; Helper poll, lease, result, ack, local
// policy, service lifecycle, and execution state are reserved for later tasks.
var helperJobs = Migration{
	Version: 51,
	Name:    "helper_job_enqueue_authority",
	Up: func(tx *gorm.DB) error {
		if err := tx.Exec(`CREATE TABLE IF NOT EXISTS helper_jobs (
  id                         TEXT    PRIMARY KEY,
  owner_user_id              TEXT    NOT NULL,
  org_id                     TEXT    NOT NULL,
  enrollment_id              TEXT    NOT NULL,
  helper_device_id           TEXT,
  job_type                   TEXT    NOT NULL,
  category                   TEXT    NOT NULL,
  schema_version             INTEGER NOT NULL,
  payload_json               TEXT    NOT NULL,
  payload_hash               TEXT    NOT NULL,
  manifest_digest            TEXT,
  manifest_binding_json      TEXT,
  idempotency_key            TEXT,
  idempotency_scope          TEXT    NOT NULL,
  active_idempotency_scope   TEXT,
  status                     TEXT    NOT NULL CHECK (status IN ('queued','leased','running','succeeded','failed','cancelled','expired')),
  failure_code               TEXT,
  failure_message            TEXT,
  created_at                 INTEGER NOT NULL,
  updated_at                 INTEGER NOT NULL,
  expires_at                 INTEGER NOT NULL,
  leased_at                  INTEGER,
  lease_expires_at           INTEGER,
  completed_at               INTEGER,
  result_summary_json        TEXT,
  FOREIGN KEY(enrollment_id) REFERENCES helper_enrollments(id)
)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_helper_jobs_owner_org
  ON helper_jobs(owner_user_id, org_id, created_at DESC)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_helper_jobs_enrollment_status
  ON helper_jobs(enrollment_id, status, expires_at)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_helper_jobs_status_expiry
  ON helper_jobs(status, expires_at)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_helper_jobs_active_idempotency_scope
  ON helper_jobs(active_idempotency_scope)
  WHERE active_idempotency_scope IS NOT NULL`).Error; err != nil {
			return err
		}
		return nil
	},
}
