// WebSocket connection
let ws;
let reconnectInterval = 3000;
let currentFilter = 'all';
let allJobs = [];

// Pagination state
let currentPage = 1;
let pageSize = 25;
let totalPages = 1;

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    setupWebSocket();
    setupEventListeners();
    fetchInitialData();
});

// Setup WebSocket connection
function setupWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws`;
    
    ws = new WebSocket(wsUrl);
    
    ws.onopen = () => {
        console.log('WebSocket connected');
        updateConnectionStatus(true);
    };
    
    ws.onmessage = (event) => {
        const data = JSON.parse(event.data);
        updateDashboard(data);
    };
    
    ws.onclose = () => {
        console.log('WebSocket disconnected');
        updateConnectionStatus(false);
        // Reconnect after delay
        setTimeout(setupWebSocket, reconnectInterval);
    };
    
    ws.onerror = (error) => {
        console.error('WebSocket error:', error);
        updateConnectionStatus(false);
    };
}

// Update connection status indicator
function updateConnectionStatus(connected) {
    const statusCard = document.getElementById('connection-status');
    const statusText = document.getElementById('ws-status');
    
    if (connected) {
        statusCard.classList.add('connected');
        statusCard.classList.remove('disconnected');
        statusText.textContent = 'Connected';
    } else {
        statusCard.classList.add('disconnected');
        statusCard.classList.remove('connected');
        statusText.textContent = 'Disconnected';
    }
}

// Fetch initial data
async function fetchInitialData() {
    try {
        const [jobsResponse, metricsResponse] = await Promise.all([
            fetch('/api/jobs'),
            fetch('/api/metrics')
        ]);
        
        const jobs = await jobsResponse.json();
        const metrics = await metricsResponse.json();
        
        updateDashboard({ jobs, metrics });
    } catch (error) {
        console.error('Failed to fetch initial data:', error);
    }
}

// Update dashboard with new data
function updateDashboard(data) {
    if (data.metrics) {
        updateMetrics(data.metrics);
    }
    
    if (data.jobs) {
        allJobs = data.jobs;
        renderJobs();
        renderDLQ();
    }
}

// Update metrics display
function updateMetrics(metrics) {
    document.getElementById('metric-total').textContent = metrics.total_jobs || 0;
    document.getElementById('metric-pending').textContent = metrics.pending_jobs || 0;
    document.getElementById('metric-running').textContent = metrics.running_jobs || 0;
    document.getElementById('metric-completed').textContent = metrics.completed_jobs || 0;
    document.getElementById('metric-failed').textContent = metrics.failed_jobs || 0;
    document.getElementById('metric-dlq').textContent = metrics.dlq_jobs || 0;
    document.getElementById('metric-retries').textContent = metrics.total_retries || 0;
}

// Render jobs based on current filter with pagination
function renderJobs() {
    const container = document.getElementById('jobs-container');
    
    let filteredJobs = allJobs;
    
    // Filter out DLQ jobs from main view
    filteredJobs = filteredJobs.filter(job => 
        !(job.status === 'failed' && job.retry_count >= job.max_retries)
    );
    
    // Apply status filter
    if (currentFilter !== 'all') {
        filteredJobs = filteredJobs.filter(job => job.status === currentFilter);
    }
    
    // Calculate pagination
    const totalJobs = filteredJobs.length;
    totalPages = Math.ceil(totalJobs / pageSize);
    
    // Ensure current page is valid
    if (currentPage > totalPages && totalPages > 0) {
        currentPage = totalPages;
    }
    if (currentPage < 1) {
        currentPage = 1;
    }
    
    // Get jobs for current page
    const startIndex = (currentPage - 1) * pageSize;
    const endIndex = startIndex + pageSize;
    const paginatedJobs = filteredJobs.slice(startIndex, endIndex);
    
    // Update job count display
    const jobsCount = document.getElementById('jobs-count');
    if (totalJobs === 0) {
        jobsCount.textContent = 'No jobs to display';
        container.innerHTML = '<div class="empty-state">No jobs to display</div>';
    } else {
        const showing = paginatedJobs.length;
        jobsCount.textContent = `Showing ${startIndex + 1}-${startIndex + showing} of ${totalJobs} jobs`;
        container.innerHTML = paginatedJobs.map(job => createJobCard(job)).join('');
    }
    
    // Update pagination controls
    updatePaginationControls();
}

// Update pagination controls (buttons and page numbers)
function updatePaginationControls() {
    const prevBtn = document.getElementById('prev-page');
    const nextBtn = document.getElementById('next-page');
    const pagesContainer = document.getElementById('pagination-pages');
    
    // Enable/disable navigation buttons
    prevBtn.disabled = currentPage === 1;
    nextBtn.disabled = currentPage === totalPages || totalPages === 0;
    
    // Generate page numbers
    pagesContainer.innerHTML = '';
    
    if (totalPages <= 7) {
        // Show all pages if 7 or fewer
        for (let i = 1; i <= totalPages; i++) {
            pagesContainer.appendChild(createPageNumber(i));
        }
    } else {
        // Show first page
        pagesContainer.appendChild(createPageNumber(1));
        
        // Show ellipsis or pages around current
        if (currentPage > 3) {
            pagesContainer.appendChild(createEllipsis());
        }
        
        // Show pages around current page
        const start = Math.max(2, currentPage - 1);
        const end = Math.min(totalPages - 1, currentPage + 1);
        
        for (let i = start; i <= end; i++) {
            pagesContainer.appendChild(createPageNumber(i));
        }
        
        // Show ellipsis or last pages
        if (currentPage < totalPages - 2) {
            pagesContainer.appendChild(createEllipsis());
        }
        
        // Show last page
        pagesContainer.appendChild(createPageNumber(totalPages));
    }
}

// Create page number element
function createPageNumber(pageNum) {
    const span = document.createElement('span');
    span.className = 'page-number' + (pageNum === currentPage ? ' active' : '');
    span.textContent = pageNum;
    span.addEventListener('click', () => {
        currentPage = pageNum;
        renderJobs();
    });
    return span;
}

// Create ellipsis element
function createEllipsis() {
    const span = document.createElement('span');
    span.className = 'page-ellipsis';
    span.textContent = '...';
    return span;
}

// Render Dead Letter Queue
function renderDLQ() {
    const container = document.getElementById('dlq-container');
    
    const dlqJobs = allJobs.filter(job => 
        job.status === 'failed' && job.retry_count >= job.max_retries
    );
    
    if (dlqJobs.length === 0) {
        container.innerHTML = '<div class="empty-state">No jobs in DLQ</div>';
        return;
    }
    
    container.innerHTML = dlqJobs.map(job => createJobCard(job, true)).join('');
}

// Create job card HTML
function createJobCard(job, isDLQ = false) {
    const createdDate = new Date(job.created_at).toLocaleString();
    const updatedDate = new Date(job.updated_at).toLocaleString();
    
    return `
        <div class="job-card status-${job.status}">
            <div class="job-header">
                <span class="job-id">${job.id}</span>
                <span class="job-status ${job.status}">${job.status}</span>
            </div>
            <div class="job-details">
                <div class="job-detail">
                    <strong>Tenant ID:</strong>
                    <span>${escapeHtml(job.tenant_id)}</span>
                </div>
                <div class="job-detail">
                    <strong>Trace ID:</strong>
                    <span>${escapeHtml(job.trace_id)}</span>
                </div>
                <div class="job-detail">
                    <strong>Payload:</strong>
                </div>
                <div class="job-payload">${escapeHtml(job.payload)}</div>
                ${job.idempotency_key ? `
                <div class="job-detail">
                    <strong>Idempotency Key:</strong>
                    <span>${escapeHtml(job.idempotency_key)}</span>
                </div>
                ` : ''}
                <div class="job-detail">
                    <strong>Retries:</strong>
                    <span>${job.retry_count} / ${job.max_retries}</span>
                </div>
                <div class="job-detail">
                    <strong>Created:</strong>
                    <span>${createdDate}</span>
                </div>
                <div class="job-detail">
                    <strong>Updated:</strong>
                    <span>${updatedDate}</span>
                </div>
                ${job.error_message ? `
                <div class="job-error">
                    <strong>Error:</strong> ${escapeHtml(job.error_message)}
                </div>
                ` : ''}
                ${isDLQ ? '<div class="job-error"><strong>⚠️ This job is in the Dead Letter Queue</strong></div>' : ''}
            </div>
        </div>
    `;
}

// Setup event listeners
function setupEventListeners() {
    // Job submission form
    document.getElementById('job-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        await submitJob();
    });
    
    // Filter buttons
    document.querySelectorAll('.filter-btn').forEach(btn => {
        btn.addEventListener('click', (e) => {
            // Update active button
            document.querySelectorAll('.filter-btn').forEach(b => b.classList.remove('active'));
            e.target.classList.add('active');
            
            // Update filter and reset to page 1
            currentFilter = e.target.dataset.filter;
            currentPage = 1;
            renderJobs();
        });
    });
    
    // Page size selector
    document.getElementById('page-size').addEventListener('change', (e) => {
        pageSize = parseInt(e.target.value);
        currentPage = 1; // Reset to first page
        renderJobs();
    });
    
    // Previous page button
    document.getElementById('prev-page').addEventListener('click', () => {
        if (currentPage > 1) {
            currentPage--;
            renderJobs();
            scrollToJobSection();
        }
    });
    
    // Next page button
    document.getElementById('next-page').addEventListener('click', () => {
        if (currentPage < totalPages) {
            currentPage++;
            renderJobs();
            scrollToJobSection();
        }
    });
}

// Scroll to job section when changing pages
function scrollToJobSection() {
    const jobsSection = document.querySelector('.jobs-section');
    if (jobsSection) {
        jobsSection.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
}

// Submit new job
async function submitJob() {
    const tenantId = document.getElementById('tenant-id').value;
    const payload = document.getElementById('payload').value;
    const idempotencyKey = document.getElementById('idempotency-key').value;
    const maxRetries = parseInt(document.getElementById('max-retries').value);
    
    const messageDiv = document.getElementById('submit-message');
    
    try {
        const response = await fetch('/api/jobs', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                tenant_id: tenantId,
                payload: payload,
                idempotency_key: idempotencyKey || undefined,
                max_retries: maxRetries
            })
        });
        
        if (response.ok) {
            const job = await response.json();
            showMessage('success', `Job submitted successfully! Job ID: ${job.id}`);
            
            // Clear form
            document.getElementById('payload').value = '';
            document.getElementById('idempotency-key').value = '';
            
            // Refresh data
            fetchInitialData();
        } else {
            const error = await response.text();
            showMessage('error', `Failed to submit job: ${error}`);
        }
    } catch (error) {
        showMessage('error', `Error: ${error.message}`);
    }
}

// Show message
function showMessage(type, text) {
    const messageDiv = document.getElementById('submit-message');
    messageDiv.className = `message ${type}`;
    messageDiv.textContent = text;
    messageDiv.style.display = 'block';
    
    setTimeout(() => {
        messageDiv.style.display = 'none';
    }, 5000);
}

// Escape HTML to prevent XSS
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Polling fallback if WebSocket fails
setInterval(() => {
    if (ws.readyState !== WebSocket.OPEN) {
        fetchInitialData();
    }
}, 5000);

