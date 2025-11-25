package worker

import (
	"context"
	"database/sql"
	"distributed-task-queue/internal/database"
	"distributed-task-queue/internal/models"
	"log"
	"time"
)

// Worker processes jobs from the queue
type Worker struct {
	id       int
	db       *database.DB
	pollTime time.Duration
	ctx      context.Context
	onUpdate func() // Callback for broadcasting updates
}

// New creates a new worker
func New(id int, db *database.DB, pollTime time.Duration, ctx context.Context, onUpdate func()) *Worker {
	return &Worker{
		id:       id,
		db:       db,
		pollTime: pollTime,
		ctx:      ctx,
		onUpdate: onUpdate,
	}
}

// Start starts the worker
func (w *Worker) Start() {
	log.Printf("[WORKER-%d] Started", w.id)

	ticker := time.NewTicker(w.pollTime)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			log.Printf("[WORKER-%d] Shutting down", w.id)
			return
		case <-ticker.C:
			w.processNextJob()
		}
	}
}

// processNextJob leases and processes a job
func (w *Worker) processNextJob() {
	now := time.Now()
	leaseDuration := 30 * time.Second
	leaseUntil := now.Add(leaseDuration)

	// Lease a job
	job, err := w.db.LeaseJob(leaseUntil)
	if err == sql.ErrNoRows {
		return // No jobs available
	}
	if err != nil {
		log.Printf("[WORKER-%d] Failed to lease job: %v", w.id, err)
		return
	}

	log.Printf("[START] TraceID=%s JobID=%s WorkerID=%d Status=running", job.TraceID, job.ID, w.id)
	if w.onUpdate != nil {
		w.onUpdate()
	}

	// Process the job
	success := w.executeJob(job)

	// Acknowledge or retry the job
	if success {
		err = w.db.UpdateJobStatus(job.ID, models.StatusDone, "")
		log.Printf("[FINISH] TraceID=%s JobID=%s WorkerID=%d Status=done", job.TraceID, job.ID, w.id)
	} else {
		job.RetryCount++
		if job.RetryCount >= job.MaxRetries {
			// Move to DLQ
			err = w.db.UpdateJobStatus(job.ID, models.StatusFailed, "Max retries exceeded - moved to DLQ")
			log.Printf("[DLQ] TraceID=%s JobID=%s WorkerID=%d Status=failed RetryCount=%d",
				job.TraceID, job.ID, w.id, job.RetryCount)
		} else {
			// Retry
			err = w.db.UpdateJobForRetry(job.ID, job.RetryCount, "Job failed - will retry")
			log.Printf("[RETRY] TraceID=%s JobID=%s WorkerID=%d RetryCount=%d/%d",
				job.TraceID, job.ID, w.id, job.RetryCount, job.MaxRetries)
		}
	}

	if err != nil {
		log.Printf("[ERROR] TraceID=%s Failed to update job status: %v", job.TraceID, err)
	}

	if w.onUpdate != nil {
		w.onUpdate()
	}
}

// executeJob simulates job execution
func (w *Worker) executeJob(job *models.Job) bool {
	// Simulate work (2-5 seconds)
	duration := time.Duration(2+time.Now().Unix()%3) * time.Second
	log.Printf("[EXECUTE] TraceID=%s JobID=%s WorkerID=%d Payload=%s Duration=%v",
		job.TraceID, job.ID, w.id, job.Payload, duration)

	time.Sleep(duration)

	// Simulate 20% failure rate for demonstration
	success := time.Now().Unix()%5 != 0

	return success
}

