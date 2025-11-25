package api

import (
	"distributed-task-queue/internal/database"
	"distributed-task-queue/internal/models"
	"distributed-task-queue/internal/ratelimit"
	"distributed-task-queue/internal/websocket"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	ws "github.com/gorilla/websocket"
)

// Server holds all HTTP handlers and dependencies
type Server struct {
	db          *database.DB
	rateLimiter *ratelimit.RateLimiter
	wsManager   *websocket.Manager
	upgrader    ws.Upgrader
}

// NewServer creates a new API server
func NewServer(db *database.DB, wsManager *websocket.Manager) *Server {
	return &Server{
		db:          db,
		rateLimiter: ratelimit.New(10), // 10 jobs per minute
		wsManager:   wsManager,
		upgrader: ws.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// SubmitJob handles job submission
func (s *Server) SubmitJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.JobSubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.TenantID == "" || req.Payload == "" {
		http.Error(w, "tenant_id and payload are required", http.StatusBadRequest)
		return
	}

	// Rate limiting check
	if !s.rateLimiter.Allow(req.TenantID) {
		log.Printf("[RATE_LIMIT] Tenant %s exceeded rate limit", req.TenantID)
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	// Check concurrent jobs quota
	runningCount, err := s.db.GetRunningJobsCount(req.TenantID)
	if err != nil {
		log.Printf("[ERROR] Failed to check concurrent jobs: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if runningCount >= 5 {
		log.Printf("[QUOTA] Tenant %s exceeded concurrent job limit", req.TenantID)
		http.Error(w, "Concurrent job limit exceeded (max 5)", http.StatusTooManyRequests)
		return
	}

	// Check idempotency
	if req.IdempotencyKey != "" {
		existingJob, err := s.db.GetJobByIdempotencyKey(req.IdempotencyKey)
		if err == nil {
			// Job already exists
			log.Printf("[IDEMPOTENCY] Job with key %s already exists: %s", req.IdempotencyKey, existingJob.ID)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(existingJob)
			return
		}
	}

	// Create new job
	maxRetries := req.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}

	traceID := fmt.Sprintf("trace-%d", time.Now().UnixNano())
	jobID := fmt.Sprintf("job-%d", time.Now().UnixNano())
	now := time.Now()

	job := &models.Job{
		ID:             jobID,
		TenantID:       req.TenantID,
		Payload:        req.Payload,
		Status:         models.StatusPending,
		IdempotencyKey: req.IdempotencyKey,
		RetryCount:     0,
		MaxRetries:     maxRetries,
		CreatedAt:      now,
		UpdatedAt:      now,
		TraceID:        traceID,
	}

	if err := s.db.InsertJob(job); err != nil {
		log.Printf("[ERROR] TraceID=%s Failed to insert job: %v", traceID, err)
		http.Error(w, "Failed to create job", http.StatusInternalServerError)
		return
	}

	log.Printf("[SUBMIT] TraceID=%s JobID=%s TenantID=%s Status=pending", traceID, jobID, req.TenantID)

	s.wsManager.Broadcast()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(job)
}

// GetJobStatus returns job status
func (s *Server) GetJobStatus(w http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Query().Get("id")
	if jobID == "" {
		http.Error(w, "job id is required", http.StatusBadRequest)
		return
	}

	job, err := s.db.GetJobByID(jobID)
	if err != nil {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

// ListJobs returns all jobs
func (s *Server) ListJobs(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	tenantID := r.URL.Query().Get("tenant_id")

	jobs, err := s.db.ListJobs(status, tenantID, 100)
	if err != nil {
		log.Printf("[ERROR] Failed to query jobs: %v", err)
		http.Error(w, "Failed to fetch jobs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}

// GetMetrics returns system metrics
func (s *Server) GetMetrics(w http.ResponseWriter, r *http.Request) {
	metrics, err := s.db.GetMetrics()
	if err != nil {
		log.Printf("[ERROR] Failed to get metrics: %v", err)
		http.Error(w, "Failed to fetch metrics", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

// HandleWebSocket handles WebSocket connections
func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ERROR] WebSocket upgrade failed: %v", err)
		return
	}

	s.wsManager.AddClient(conn)
}

// SetupRoutes sets up all HTTP routes
func (s *Server) SetupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/jobs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			s.SubmitJob(w, r)
		} else if r.Method == http.MethodGet {
			s.ListJobs(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/jobs/status", s.GetJobStatus)
	mux.HandleFunc("/api/metrics", s.GetMetrics)
	mux.HandleFunc("/ws", s.HandleWebSocket)

	// Serve static files
	mux.Handle("/", http.FileServer(http.Dir("./static")))
}

