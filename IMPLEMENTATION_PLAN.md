# IABot-Go Implementation Plan

This document outlines a phased approach to incrementally add features from the official IABot.

## Current State

- Basic live/dead checking via HEAD/GET requests
- Basic Wayback availability checking via `/wayback/available` API
- No caching, no retries, no advanced heuristics
- Simple sequential processing

## Phase 1: Core Link Checking Improvements

### 1.1 Better Dead/Alive Detection
- [x] Add proper HTTP status code interpretation (2xx=alive, 4xx/5xx=dead, with exceptions)
- [x] Handle redirects explicitly (follow and record final destination)
- [ ] Add soft-404 detection (page returns 200 but content indicates "not found")
- [x] Add DNS/TLS error handling and reporting
- [x] Record detailed error information (not just "unknown")

### 1.2 Enhanced Archive Checking
- [x] Query Wayback with `statuscodes=200,203,206` parameter (only accept good snapshots)
- [x] Parse and validate archive timestamps (reject pre-1996 or future dates)
- [ ] Implement "closest before" and "closest after" logic for finding best snapshot
- [x] Detect existing archive URLs in input (don't check archives for archives)

### 1.3 Better Error Handling
- [x] Add timeout handling with proper error messages
- [x] Log request details (HTTP codes, timing, headers)
- [ ] Return structured error information to UI

**Priority:** HIGH - These are foundational improvements to core functionality

**Status:** MOSTLY COMPLETE (7/10 items done - soft-404 detection, closest before/after, and structured error UI remain)

---

## Phase 2: Archive Detection & Normalization

### 2.1 Archive URL Detection
- [ ] Detect Wayback URLs (web.archive.org)
- [ ] Detect archive.today/is URLs
- [ ] Detect WebCite, Memento, Archive-It
- [ ] Detect national archives (UK Web Archive, etc.)
- [ ] Extract original URL and timestamp from archive URLs

### 2.2 Archive Validation
- [ ] Validate archive URL structure
- [ ] Check timestamp validity (1996-03-01 to present)
- [ ] Handle nested archive URLs (archive of archive)
- [ ] Mark invalid archives for correction

### 2.3 Archive Normalization
- [ ] Canonicalize archive URLs to preferred format
- [ ] Convert between archive hosts when appropriate
- [ ] Preserve URL fragments correctly

**Priority:** MEDIUM - Important for handling existing archive links correctly

---

## Phase 3: Database & Caching Layer

### 3.1 Database Schema
- [ ] Design schema for storing link check results
- [ ] Tables: `links`, `archive_snapshots`, `whitelist`, `false_positives`
- [ ] Fields: `live_state`, `archived`, `archivable`, `has_archive`, `paywall_status`

### 3.2 Caching Logic
- [ ] Cache live/dead results (with expiration)
- [ ] Cache archive availability results
- [ ] Skip re-checking recently verified URLs
- [ ] Record "unarchivable" URLs to avoid retry loops

### 3.3 Persistence
- [ ] Choose database (SQLite for local, PostgreSQL for production)
- [ ] Implement data access layer
- [ ] Add migration system

**Priority:** MEDIUM - Needed for performance and avoiding API rate limits

---

## Phase 4: Retry, Throttling & Backoff

### 4.1 Retry Logic
- [ ] Retry on 429 (rate limit) with `Retry-After` header respect
- [ ] Retry on 5xx errors with exponential backoff
- [ ] Retry on transient errors (408 timeout, 503 service unavailable)
- [ ] Configurable max retry attempts per URL

### 4.2 Throttling
- [ ] Track API call rate to Wayback (respect limits)
- [ ] Add sliding window rate limiter
- [ ] Queue requests when approaching limits
- [ ] Global 429 handling (pause all requests)

### 4.3 Batch Processing
- [ ] Batch Wayback availability checks (CDX API supports multiple URLs)
- [ ] Process results with original URL mapping
- [ ] Handle partial failures in batch

**Priority:** MEDIUM - Important for reliability and being a good API citizen

---

## Phase 5: Advanced Archive Operations

### 5.1 Save Page Now (SPN) Integration
- [ ] Implement SPN API client with authentication
- [ ] Request new snapshots for dead links
- [ ] Handle SPN errors (user-session-limit, celery errors)
- [ ] Mark URLs as "not archivable" on permanent failures

### 5.2 Smart Snapshot Selection
- [ ] Find closest snapshot before link death date
- [ ] Fall back to closest after if no before exists
- [ ] Prefer 200 status over 203/206
- [ ] Validate snapshot actually contains content

### 5.3 Outlinks Capture
- [ ] Request outlinks capture during SPN
- [ ] Store related links for future processing

**Priority:** LOW - Advanced feature, requires authentication

---

## Phase 6: False Positive Handling & Whitelisting

### 6.1 Multi-Source Validation
- [ ] Configure additional "check-if-dead" servers
- [ ] Get second opinion on "dead" results
- [ ] Whitelist URLs that pass second-opinion check
- [ ] Record which server disagreed (for debugging)

### 6.2 Domain Whitelisting
- [ ] Maintain whitelist of domains with known issues
- [ ] Auto-mark whitelisted domains as "alive"
- [ ] Support paywall detection/marking
- [ ] Admin interface for managing whitelist

### 6.3 False Positive Reporting
- [ ] Track potential false positives
- [ ] Email notification system for subscribers
- [ ] Web interface to review/confirm false positives
- [ ] Automatic revert detection

**Priority:** LOW - Quality-of-life feature for reducing false positives

---

## Phase 7: User Interface Improvements

### 7.1 Enhanced Results Display
- [ ] Show detailed status (not just "unknown")
- [ ] Display error messages (DNS failed, timeout, etc.)
- [ ] Show redirect chains
- [ ] Indicate whitelisted/cached results

### 7.2 Real-time Progress
- [ ] WebSocket or SSE for live progress updates
- [ ] Progress bar showing X/Y links checked
- [ ] Stream results as they complete
- [ ] Cancel/pause functionality

### 7.3 Bulk Operations
- [ ] Upload list of URLs to check
- [ ] Download results as CSV/JSON
- [ ] Batch archive requests
- [ ] Compare snapshots over time

**Priority:** MEDIUM - Better UX for users

---

## Phase 8: Production Readiness

### 8.1 Monitoring & Observability
- [ ] Structured logging (JSON format)
- [ ] Metrics (Prometheus/StatsD)
- [ ] Health check endpoint
- [ ] Performance monitoring

### 8.2 Error Reporting
- [ ] Sentry integration for critical errors
- [ ] Email alerts for API failures
- [ ] Dashboard for error trends
- [ ] Automatic issue creation for recurring errors

### 8.3 Configuration Management
- [ ] Environment-based config (dev/staging/prod)
- [ ] Feature flags for gradual rollout
- [ ] Runtime config updates (no restart)
- [ ] Config validation at startup

**Priority:** HIGH (when approaching production)

---

## Implementation Strategy

1. **Start with Phase 1** - Core improvements to what we already have
2. **Add logging/monitoring early** - Will help debug later phases
3. **Add database before advanced features** - Caching is crucial for performance
4. **Implement features incrementally** - Each checkbox can be a separate PR
5. **Test thoroughly** - Each phase should have integration tests
6. **Document as we go** - Keep README updated with new features

## Next Steps

Which phase should we start with? I recommend:

**Option A (Quick Wins):** Phase 1.1 & 1.2 - Better detection logic (no new dependencies)
**Option B (Foundation):** Phase 3 - Add database for caching (enables many later features)
**Option C (UX First):** Phase 7.2 - Real-time progress (makes current tool more usable)

What would you like to tackle first?
