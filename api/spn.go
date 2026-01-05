package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// SPNJob represents a pending or completed archive request
type SPNJob struct {
	URL       string `json:"url"`
	JobID     string `json:"job_id"`
	Status    string `json:"status"` // "pending", "success", "error"
	Timestamp string `json:"timestamp,omitempty"`
	Error     string `json:"error,omitempty"`
}

// SPNSubmitRequest is the request body for submitting URLs
type SPNSubmitRequest struct {
	URLs      []string `json:"urls"`
	AccessKey string   `json:"access_key"`
	SecretKey string   `json:"secret_key"`
}

// SPNSubmitResponse is the response for a submission
type SPNSubmitResponse struct {
	Submitted []SPNJob `json:"submitted"`
	Errors    []string `json:"errors,omitempty"`
}

// Rate limiter for SPN API (10 seconds between requests = 6/min)
type spnRateLimiter struct {
	mu          sync.Mutex
	lastRequest time.Time
	minInterval time.Duration
}

var spnLimiter = &spnRateLimiter{minInterval: 10 * time.Second}

func (rl *spnRateLimiter) wait(ctx context.Context) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	elapsed := time.Since(rl.lastRequest)
	if elapsed < rl.minInterval {
		wait := rl.minInterval - elapsed
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	rl.lastRequest = time.Now()
	return nil
}

// SPNSubmitHandler handles POST /api/spn/submit
func SPNSubmitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SPNSubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate credentials
	if req.AccessKey == "" || req.SecretKey == "" {
		http.Error(w, "Credentials required", http.StatusBadRequest)
		return
	}

	if len(req.URLs) == 0 {
		http.Error(w, "No URLs provided", http.StatusBadRequest)
		return
	}

	// Limit batch size
	if len(req.URLs) > 10 {
		req.URLs = req.URLs[:10]
	}

	resp := SPNSubmitResponse{
		Submitted: make([]SPNJob, 0, len(req.URLs)),
	}

	// Submit each URL
	for _, targetURL := range req.URLs {
		job, err := submitToSPN(r.Context(), targetURL, req.AccessKey, req.SecretKey)
		if err != nil {
			job = SPNJob{
				URL:    targetURL,
				Status: "error",
				Error:  err.Error(),
			}
		}
		resp.Submitted = append(resp.Submitted, job)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// SPNStatusHandler handles GET /api/spn/status?job_id=xxx
func SPNStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobID := r.URL.Query().Get("job_id")
	if jobID == "" {
		http.Error(w, "job_id required", http.StatusBadRequest)
		return
	}

	job, err := checkSPNStatus(r.Context(), jobID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

// submitToSPN submits a URL to the Wayback Machine's Save Page Now API
func submitToSPN(ctx context.Context, targetURL, accessKey, secretKey string) (SPNJob, error) {
	job := SPNJob{URL: targetURL}

	// Wait for rate limiter
	if err := spnLimiter.wait(ctx); err != nil {
		return job, fmt.Errorf("rate limit wait cancelled: %w", err)
	}

	// Build form data
	form := url.Values{}
	form.Set("url", targetURL)
	form.Set("capture_all", "1") // Capture even error pages

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	log.Printf("[SPN] Submitting URL: %s", targetURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://web.archive.org/save", strings.NewReader(form.Encode()))
	if err != nil {
		return job, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("LOW %s:%s", accessKey, secretKey))
	req.Header.Set("User-Agent", "IABot-Go/0.1 (+https://github.com/comaeclipse/IABot-Go)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[SPN] Request failed for %s: %v", targetURL, err)
		return job, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	log.Printf("[SPN] Response status: %d, body: %s", resp.StatusCode, string(body))

	// Handle rate limiting
	if resp.StatusCode == 429 {
		return job, fmt.Errorf("rate limited, try again later")
	}

	// Handle auth errors
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return job, fmt.Errorf("invalid credentials")
	}

	if resp.StatusCode != http.StatusOK {
		return job, fmt.Errorf("SPN error: HTTP %d", resp.StatusCode)
	}

	// Parse response
	var spnResp struct {
		URL       string `json:"url"`
		JobID     string `json:"job_id"`
		Message   string `json:"message"`
		Status    string `json:"status"`
		Timestamp string `json:"timestamp"`
	}
	if err := json.Unmarshal(body, &spnResp); err != nil {
		// Sometimes SPN returns HTML or non-JSON on success
		log.Printf("[SPN] JSON decode error, treating as pending: %v", err)
		job.Status = "pending"
		return job, nil
	}

	job.JobID = spnResp.JobID
	job.Timestamp = spnResp.Timestamp

	// Determine status
	if spnResp.Status != "" {
		job.Status = spnResp.Status
	} else if spnResp.JobID != "" {
		job.Status = "pending"
	} else if spnResp.Timestamp != "" {
		job.Status = "success"
	} else {
		job.Status = "pending"
	}

	log.Printf("[SPN] Submitted %s: job_id=%s, status=%s", targetURL, job.JobID, job.Status)
	return job, nil
}

// checkSPNStatus checks the status of a SPN job
func checkSPNStatus(ctx context.Context, jobID string) (SPNJob, error) {
	var job SPNJob
	job.JobID = jobID

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	reqURL := "https://web.archive.org/save/status/" + url.PathEscape(jobID)
	log.Printf("[SPN] Checking status: %s", reqURL)

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "IABot-Go/0.1 (+https://github.com/comaeclipse/IABot-Go)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return job, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	log.Printf("[SPN] Status response: %d, body: %s", resp.StatusCode, string(body))

	var statusResp struct {
		Status      string `json:"status"`
		Timestamp   string `json:"timestamp"`
		OriginalURL string `json:"original_url"`
		Message     string `json:"message"`
		JobID       string `json:"job_id"`
	}
	if err := json.Unmarshal(body, &statusResp); err != nil {
		return job, fmt.Errorf("invalid response from SPN")
	}

	job.URL = statusResp.OriginalURL
	job.Status = statusResp.Status
	job.Timestamp = statusResp.Timestamp

	if job.Status == "error" {
		job.Error = statusResp.Message
	}

	return job, nil
}
