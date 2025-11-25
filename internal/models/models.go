package models

import "time"

// Job represents a task in the queue
type Job struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id"`
	Payload        string     `json:"payload"`
	Status         string     `json:"status"` // pending, running, done, failed
	IdempotencyKey string     `json:"idempotency_key,omitempty"`
	RetryCount     int        `json:"retry_count"`
	MaxRetries     int        `json:"max_retries"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	LeasedUntil    *time.Time `json:"leased_until,omitempty"`
	ErrorMessage   string     `json:"error_message,omitempty"`
	TraceID        string     `json:"trace_id"`
}

// Metrics holds system metrics
type Metrics struct {
	TotalJobs     int64 `json:"total_jobs"`
	PendingJobs   int64 `json:"pending_jobs"`
	RunningJobs   int64 `json:"running_jobs"`
	CompletedJobs int64 `json:"completed_jobs"`
	FailedJobs    int64 `json:"failed_jobs"`
	DLQJobs       int64 `json:"dlq_jobs"`
	TotalRetries  int64 `json:"total_retries"`
}

// JobSubmitRequest represents a job submission request
type JobSubmitRequest struct {
	TenantID       string `json:"tenant_id"`
	Payload        string `json:"payload"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	MaxRetries     int    `json:"max_retries,omitempty"`
}

// Status constants
const (
	StatusPending = "pending"
	StatusRunning = "running"
	StatusDone    = "done"
	StatusFailed  = "failed"
)

