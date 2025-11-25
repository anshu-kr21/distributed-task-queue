package database

import (
	"database/sql"
	"distributed-task-queue/internal/models"
	"time"
)

// DB wraps the SQL database with helper methods
type DB struct {
	*sql.DB
}

// New creates a new database connection
func New(dataSourceName string) (*DB, error) {
	db, err := sql.Open("sqlite3", dataSourceName)
	if err != nil {
		return nil, err
	}
	return &DB{db}, nil
}

// InitSchema initializes the database schema
func (db *DB) InitSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS jobs (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		payload TEXT NOT NULL,
		status TEXT NOT NULL,
		idempotency_key TEXT,
		retry_count INTEGER DEFAULT 0,
		max_retries INTEGER DEFAULT 3,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		leased_until DATETIME,
		error_message TEXT,
		trace_id TEXT NOT NULL
	);
	
	CREATE INDEX IF NOT EXISTS idx_status ON jobs(status);
	CREATE INDEX IF NOT EXISTS idx_tenant ON jobs(tenant_id);
	CREATE INDEX IF NOT EXISTS idx_idempotency ON jobs(idempotency_key) WHERE idempotency_key IS NOT NULL;
	CREATE INDEX IF NOT EXISTS idx_leased ON jobs(leased_until) WHERE leased_until IS NOT NULL;
	`

	_, err := db.Exec(schema)
	return err
}

// InsertJob inserts a new job into the database
func (db *DB) InsertJob(job *models.Job) error {
	_, err := db.Exec(`
		INSERT INTO jobs (id, tenant_id, payload, status, idempotency_key, retry_count, max_retries, created_at, updated_at, trace_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, job.ID, job.TenantID, job.Payload, job.Status, nullString(job.IdempotencyKey),
		job.RetryCount, job.MaxRetries, job.CreatedAt, job.UpdatedAt, job.TraceID)
	return err
}

// GetJobByID retrieves a job by its ID
func (db *DB) GetJobByID(id string) (*models.Job, error) {
	var job models.Job
	var leasedUntil sql.NullTime
	var idempotencyKey sql.NullString
	var errorMessage sql.NullString

	err := db.QueryRow(`
		SELECT id, tenant_id, payload, status, idempotency_key, retry_count, max_retries, 
		       created_at, updated_at, leased_until, error_message, trace_id
		FROM jobs WHERE id = ?
	`, id).Scan(&job.ID, &job.TenantID, &job.Payload, &job.Status,
		&idempotencyKey, &job.RetryCount, &job.MaxRetries,
		&job.CreatedAt, &job.UpdatedAt, &leasedUntil, &errorMessage, &job.TraceID)

	if err != nil {
		return nil, err
	}

	if idempotencyKey.Valid {
		job.IdempotencyKey = idempotencyKey.String
	}
	if leasedUntil.Valid {
		t := leasedUntil.Time
		job.LeasedUntil = &t
	}
	if errorMessage.Valid {
		job.ErrorMessage = errorMessage.String
	}

	return &job, nil
}

// GetJobByIdempotencyKey retrieves a job by its idempotency key
func (db *DB) GetJobByIdempotencyKey(key string) (*models.Job, error) {
	var id string
	err := db.QueryRow("SELECT id FROM jobs WHERE idempotency_key = ?", key).Scan(&id)
	if err != nil {
		return nil, err
	}
	return db.GetJobByID(id)
}

// ListJobs retrieves jobs with optional filtering
func (db *DB) ListJobs(status, tenantID string, limit int) ([]models.Job, error) {
	query := `SELECT id, tenant_id, payload, status, idempotency_key, retry_count, max_retries,
	          created_at, updated_at, leased_until, error_message, trace_id
	          FROM jobs WHERE 1=1`
	args := []interface{}{}

	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}

	if tenantID != "" {
		query += " AND tenant_id = ?"
		args = append(args, tenantID)
	}

	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanJobs(rows)
}

// GetAllJobs retrieves all jobs
func (db *DB) GetAllJobs() ([]models.Job, error) {
	rows, err := db.Query(`
		SELECT id, tenant_id, payload, status, idempotency_key, retry_count, max_retries,
		       created_at, updated_at, leased_until, error_message, trace_id
		FROM jobs ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanJobs(rows)
}

// GetRunningJobsCount returns the count of running jobs for a tenant
func (db *DB) GetRunningJobsCount(tenantID string) (int, error) {
	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM jobs WHERE tenant_id = ? AND status = ?",
		tenantID, models.StatusRunning,
	).Scan(&count)
	return count, err
}

// UpdateJobStatus updates a job's status
func (db *DB) UpdateJobStatus(jobID, status string, errorMsg string) error {
	_, err := db.Exec(`
		UPDATE jobs 
		SET status = ?, updated_at = ?, leased_until = NULL, error_message = ?
		WHERE id = ?
	`, status, time.Now(), nullString(errorMsg), jobID)
	return err
}

// UpdateJobForRetry updates a job for retry
func (db *DB) UpdateJobForRetry(jobID string, retryCount int, errorMsg string) error {
	_, err := db.Exec(`
		UPDATE jobs 
		SET status = ?, retry_count = ?, updated_at = ?, leased_until = NULL, error_message = ?
		WHERE id = ?
	`, models.StatusFailed, retryCount, time.Now(), errorMsg, jobID)
	return err
}

// LeaseJob atomically leases a job for processing
func (db *DB) LeaseJob(leaseUntil time.Time) (*models.Job, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	now := time.Now()
	var jobID, tenantID, payload, status, traceID string
	var retryCount, maxRetries int

	// Try to get a job that needs processing
	err = tx.QueryRow(`
		SELECT id, tenant_id, payload, status, retry_count, max_retries, trace_id
		FROM jobs
		WHERE (status = ? OR 
		       (status = ? AND leased_until < ?) OR
		       (status = ? AND retry_count < max_retries))
		ORDER BY created_at ASC
		LIMIT 1
	`, models.StatusPending, models.StatusRunning, now, models.StatusFailed).Scan(
		&jobID, &tenantID, &payload, &status, &retryCount, &maxRetries, &traceID)

	if err != nil {
		return nil, err
	}

	// Lease the job
	_, err = tx.Exec(`
		UPDATE jobs 
		SET status = ?, leased_until = ?, updated_at = ?
		WHERE id = ?
	`, models.StatusRunning, leaseUntil, now, jobID)

	if err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	// Return the leased job
	return &models.Job{
		ID:          jobID,
		TenantID:    tenantID,
		Payload:     payload,
		Status:      models.StatusRunning,
		RetryCount:  retryCount,
		MaxRetries:  maxRetries,
		TraceID:     traceID,
		LeasedUntil: &leaseUntil,
	}, nil
}

// GetMetrics retrieves system metrics
func (db *DB) GetMetrics() (*models.Metrics, error) {
	var metrics models.Metrics

	db.QueryRow("SELECT COUNT(*) FROM jobs").Scan(&metrics.TotalJobs)
	db.QueryRow("SELECT COUNT(*) FROM jobs WHERE status = ?", models.StatusPending).Scan(&metrics.PendingJobs)
	db.QueryRow("SELECT COUNT(*) FROM jobs WHERE status = ?", models.StatusRunning).Scan(&metrics.RunningJobs)
	db.QueryRow("SELECT COUNT(*) FROM jobs WHERE status = ?", models.StatusDone).Scan(&metrics.CompletedJobs)
	db.QueryRow("SELECT COUNT(*) FROM jobs WHERE status = ? AND retry_count < max_retries", models.StatusFailed).Scan(&metrics.FailedJobs)
	db.QueryRow("SELECT COUNT(*) FROM jobs WHERE status = ? AND retry_count >= max_retries", models.StatusFailed).Scan(&metrics.DLQJobs)
	db.QueryRow("SELECT COALESCE(SUM(retry_count), 0) FROM jobs").Scan(&metrics.TotalRetries)

	return &metrics, nil
}

// Helper functions

func scanJobs(rows *sql.Rows) ([]models.Job, error) {
	jobs := []models.Job{}
	for rows.Next() {
		var job models.Job
		var leasedUntil sql.NullTime
		var idempotencyKey sql.NullString
		var errorMessage sql.NullString

		err := rows.Scan(&job.ID, &job.TenantID, &job.Payload, &job.Status,
			&idempotencyKey, &job.RetryCount, &job.MaxRetries,
			&job.CreatedAt, &job.UpdatedAt, &leasedUntil, &errorMessage, &job.TraceID)

		if err != nil {
			continue
		}

		if idempotencyKey.Valid {
			job.IdempotencyKey = idempotencyKey.String
		}
		if leasedUntil.Valid {
			t := leasedUntil.Time
			job.LeasedUntil = &t
		}
		if errorMessage.Valid {
			job.ErrorMessage = errorMessage.String
		}

		jobs = append(jobs, job)
	}
	return jobs, nil
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

