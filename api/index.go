package handler

import (
    "context"
    "embed"
    "encoding/json"
    "fmt"
    "html/template"
    "io"
    "log"
    "net/http"
    "net/url"
    "regexp"
    "sort"
    "strings"
    "time"
)

//go:embed templates/index.html
var tmplFS embed.FS

type pageData struct {
    Title     string
    Message   string
    Query     string
    Results   []linkResult
    Citations []Citation // Citations with URLs for citation-first view
    ViewMode  string     // "url" or "citation"
    Error     string
}

type linkResult struct {
    URL             string
    LiveCode        int
    LiveStatus      string
    Archived        bool
    ArchiveURL      string
    ArchiveStatus   string
    CitationNumbers []int // Which citations reference this URL
}

type apiError struct {
    msg     string
    status  int
    payload string
}

func (e *apiError) Error() string {
    parts := []string{e.msg}
    if e.status != 0 {
        parts = append(parts, http.StatusText(e.status))
    }
    if e.payload != "" {
        parts = append(parts, e.payload)
    }
    return strings.Join(parts, ": ")
}

// Citation represents a single <ref> tag in the wikitext
type Citation struct {
	Number int      // Assigned citation number (1-based)
	Name   string   // ref name attribute (empty if unnamed)
	URLs   []string // Extracted URLs from this citation
}

// CitationMap provides bidirectional lookup between citations and URLs
type CitationMap struct {
	Citations     []Citation       // All citations with URLs, in order
	URLToCitation map[string][]int // URL -> list of citation numbers that use it
	NameToNumber  map[string]int   // ref name -> citation number (for reuse tracking)
}

// Regex patterns for parsing
var (
	// Match <ref> tags: <ref name="foo">content</ref> or <ref name="foo"/>
	// Group 1: full name attribute, Group 2: name value, Group 3: content (if not self-closing)
	refPattern = regexp.MustCompile(`(?i)<ref(\s+name\s*=\s*["']?([^"'>\s]+)["']?)?\s*(?:/>|>([\s\S]*?)</ref>)`)

	// Match URLs directly in text
	urlPattern = regexp.MustCompile(`https?://[^\s<>"\]\|{}\[\]]+`)

	// Match URLs in templates like |url=... or |archive-url=...
	templateURLPattern = regexp.MustCompile(`\|\s*(?:url|archive-url|archiveurl)\s*=\s*([^\s\|\}]+)`)
)

// ParseCitations extracts citations from Wikipedia wikitext and builds a CitationMap
func ParseCitations(wikitext string) *CitationMap {
	cm := &CitationMap{
		Citations:     make([]Citation, 0),
		URLToCitation: make(map[string][]int),
		NameToNumber:  make(map[string]int),
	}

	matches := refPattern.FindAllStringSubmatch(wikitext, -1)
	citationNum := 0

	for _, match := range matches {
		// match[0] = full match
		// match[1] = name attribute with spaces (e.g., ' name="foo"')
		// match[2] = name value (e.g., "foo")
		// match[3] = content between <ref> and </ref> (empty for self-closing)

		name := strings.TrimSpace(match[2])
		content := match[3]

		// Handle self-closing refs that reference existing named refs
		if content == "" && name != "" {
			// This is a reuse like <ref name="foo"/> - skip, same number as original
			continue
		}

		// Handle named refs that were already defined
		if name != "" {
			if _, exists := cm.NameToNumber[name]; exists {
				// Already seen this named ref, skip
				continue
			}
		}

		// Extract URLs from the ref content
		urls := extractURLsFromContent(content)

		// Only create citation if it has URLs (per user request)
		if len(urls) == 0 {
			// Still need to track named refs without URLs for numbering purposes
			citationNum++
			if name != "" {
				cm.NameToNumber[name] = citationNum
			}
			continue
		}

		citationNum++
		citation := Citation{
			Number: citationNum,
			Name:   name,
			URLs:   urls,
		}

		if name != "" {
			cm.NameToNumber[name] = citationNum
		}

		cm.Citations = append(cm.Citations, citation)

		// Build reverse lookup: URL -> citation numbers
		for _, url := range urls {
			cm.URLToCitation[url] = append(cm.URLToCitation[url], citationNum)
		}
	}

	return cm
}

// extractURLsFromContent extracts URLs from ref content, handling both direct URLs
// and template parameters like |url=...
func extractURLsFromContent(content string) []string {
	seen := make(map[string]struct{})
	var urls []string

	// Extract direct URLs
	directMatches := urlPattern.FindAllString(content, -1)
	for _, u := range directMatches {
		u = cleanURL(u)
		if u != "" && !isIgnoredURL(u) {
			if _, ok := seen[u]; !ok {
				seen[u] = struct{}{}
				urls = append(urls, u)
			}
		}
	}

	// Extract URLs from templates (|url=...)
	templateMatches := templateURLPattern.FindAllStringSubmatch(content, -1)
	for _, match := range templateMatches {
		if len(match) > 1 {
			u := cleanURL(match[1])
			if u != "" && strings.HasPrefix(u, "http") && !isIgnoredURL(u) {
				if _, ok := seen[u]; !ok {
					seen[u] = struct{}{}
					urls = append(urls, u)
				}
			}
		}
	}

	return urls
}

// cleanURL removes trailing punctuation and normalizes the URL
func cleanURL(u string) string {
	u = strings.TrimSpace(u)

	// Remove common trailing characters that aren't part of URLs
	for strings.HasSuffix(u, ".") || strings.HasSuffix(u, ",") ||
		strings.HasSuffix(u, ";") || strings.HasSuffix(u, ":") ||
		strings.HasSuffix(u, ")") || strings.HasSuffix(u, "]") ||
		strings.HasSuffix(u, "'") || strings.HasSuffix(u, "\"") {
		u = u[:len(u)-1]
	}

	return u
}

// isIgnoredURL returns true for URLs we should skip (internal wiki links, etc.)
func isIgnoredURL(u string) bool {
	lower := strings.ToLower(u)

	// Skip Wikipedia internal links
	if strings.Contains(lower, "wikipedia.org/wiki/") ||
		strings.Contains(lower, "wikimedia.org") ||
		strings.Contains(lower, "wikidata.org") {
		return true
	}

	return false
}

// GetUniqueURLs returns all unique URLs from the citation map
func (cm *CitationMap) GetUniqueURLs() []string {
	urls := make([]string, 0, len(cm.URLToCitation))
	for url := range cm.URLToCitation {
		urls = append(urls, url)
	}
	return urls
}

// GetCitationNumbers returns the citation numbers that reference a given URL
func (cm *CitationMap) GetCitationNumbers(url string) []int {
	return cm.URLToCitation[url]
}

// Handler serves the interface page and processes scans.
func Handler(w http.ResponseWriter, r *http.Request) {
    t, err := template.ParseFS(tmplFS, "templates/index.html")
    if err != nil {
        http.Error(w, "template error", http.StatusInternalServerError)
        return
    }

    data := pageData{Title: "IABot-Go", Message: "Enter an English Wikipedia page to scan external links."}

    if r.Method == http.MethodGet {
        q := strings.TrimSpace(r.URL.Query().Get("page"))
        viewMode := r.URL.Query().Get("view")
        if viewMode == "" {
            viewMode = "url" // Default to URL view
        }
        data.ViewMode = viewMode

        if q != "" {
            data.Query = q
            results, citationMap, err := scanPage(r.Context(), q)
            if err != nil {
                data.Error = err.Error()
            } else {
                data.Results = results
                if citationMap != nil {
                    data.Citations = citationMap.Citations
                }
            }
        }
    }

    _ = t.Execute(w, data)
}

func scanPage(ctx context.Context, title string) ([]linkResult, *CitationMap, error) {
    log.Printf("[SCAN] Starting scan for page: %s", title)

    // Fetch wikitext via MediaWiki API to parse citations
    api := "https://en.wikipedia.org/w/api.php"
    v := url.Values{}
    v.Set("action", "parse")
    v.Set("page", title)
    v.Set("prop", "wikitext")
    v.Set("format", "json")
    // set origin to please CORS and some edge policies; harmless for server-side
    v.Set("origin", "*")
    reqURL := api + "?" + v.Encode()

    // Increase timeout to 5 minutes to handle all link checks
    ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
    defer cancel()

    log.Printf("[SCAN] Fetching wikitext from MediaWiki API...")
    req, _ := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
    req.Header.Set("User-Agent", "IABot-Go/0.1 (+https://github.com/comaeclipse/IABot-Go)")
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        log.Printf("[SCAN] Error fetching from MediaWiki API: %v", err)
        return nil, nil, err
    }
    defer resp.Body.Close()
    body, _ := io.ReadAll(resp.Body)
    log.Printf("[SCAN] MediaWiki API response status: %d", resp.StatusCode)

    // JSON decode for wikitext
    var parsed struct {
        Parse struct {
            Wikitext struct {
                Content string `json:"*"`
            } `json:"wikitext"`
        } `json:"parse"`
        Error any `json:"error"`
    }
    if err := json.Unmarshal(body, &parsed); err != nil {
        // include a snippet of the payload to aid debugging (common case: missing UA -> HTML/plaintext)
        snippet := string(body)
        if len(snippet) > 240 { snippet = snippet[:240] + "..." }
        log.Printf("[SCAN] Error decoding MediaWiki response: %v", err)
        return nil, nil, &apiError{msg: "mediawiki api decode", status: resp.StatusCode, payload: snippet}
    }

    // Parse citations from wikitext
    wikitext := parsed.Parse.Wikitext.Content
    log.Printf("[SCAN] Got wikitext (%d chars), parsing citations...", len(wikitext))
    citationMap := ParseCitations(wikitext)
    log.Printf("[SCAN] Found %d citations with URLs, %d unique URLs", len(citationMap.Citations), len(citationMap.URLToCitation))

    // Get unique URLs from citation map
    out := citationMap.GetUniqueURLs()
    sort.Strings(out)
    if len(out) > 50 {
        log.Printf("[SCAN] Limiting to first 50 of %d unique links", len(out))
        out = out[:50]
    } else {
        log.Printf("[SCAN] Processing %d unique links", len(out))
    }

    results := make([]linkResult, 0, len(out))
    for i, u := range out {
        // Check if context is cancelled
        select {
        case <-ctx.Done():
            log.Printf("[SCAN] Context cancelled after processing %d/%d links: %v", i, len(out), ctx.Err())
            return results, citationMap, fmt.Errorf("scan cancelled after %d links: %w", i, ctx.Err())
        default:
        }

        log.Printf("[SCAN] [%d/%d] Checking: %s", i+1, len(out), u)
        lr := linkResult{
            URL:             u,
            CitationNumbers: citationMap.GetCitationNumbers(u),
        }

        // Skip live/archive checks for URLs that are already archives
        if isArchiveURL(u) {
            lr.LiveCode = 0
            lr.LiveStatus = "archive URL (skipped)"
            lr.Archived = true
            lr.ArchiveURL = u
            lr.ArchiveStatus = "is archive"
            log.Printf("[SCAN] [%d/%d] Detected as archive URL, skipping checks", i+1, len(out))
            results = append(results, lr)
            continue
        }

        code, status := checkLive(ctx, u)
        lr.LiveCode = code
        lr.LiveStatus = status
        log.Printf("[SCAN] [%d/%d] Live check: %d %s", i+1, len(out), code, status)

        arch, aurl, astatus := checkWayback(ctx, u)
        lr.Archived = arch
        lr.ArchiveURL = aurl
        lr.ArchiveStatus = astatus
        log.Printf("[SCAN] [%d/%d] Wayback check: archived=%v status=%s", i+1, len(out), arch, astatus)

        results = append(results, lr)
    }
    log.Printf("[SCAN] Completed scan: processed %d links", len(results))
    return results, citationMap, nil
}

func checkLive(ctx context.Context, raw string) (int, string) {
    // Try HEAD then fallback to GET if HEAD returns 405 or fails
    status := "unknown"
    code := 0
    client := &http.Client{
        Timeout: 8 * time.Second,
        CheckRedirect: func(req *http.Request, via []*http.Request) error {
            // Allow up to 10 redirects (default)
            if len(via) >= 10 {
                return http.ErrUseLastResponse
            }
            return nil
        },
    }

    // HEAD
    req, err := http.NewRequestWithContext(ctx, http.MethodHead, raw, nil)
    if err != nil {
        log.Printf("[LIVE] Error creating HEAD request for %s: %v", raw, err)
        return code, classifyError(err)
    }

    resp, err := client.Do(req)
    if err != nil {
        log.Printf("[LIVE] HEAD request failed for %s: %v", raw, err)
        return code, classifyError(err)
    } else {
        code = resp.StatusCode
        status = classifyStatus(code, resp.Status)
        resp.Body.Close()
        log.Printf("[LIVE] HEAD response for %s: %d %s", raw, code, status)
        if code != http.StatusMethodNotAllowed && code != http.StatusNotImplemented {
            return code, status
        }
        log.Printf("[LIVE] HEAD returned %d, trying GET for %s", code, raw)
    }

    // GET with small range
    req2, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
    if err != nil {
        log.Printf("[LIVE] Error creating GET request for %s: %v", raw, err)
        return code, classifyError(err)
    }
    req2.Header.Set("Range", "bytes=0-0")
    resp2, err := client.Do(req2)
    if err != nil {
        log.Printf("[LIVE] GET request failed for %s: %v", raw, err)
        return code, classifyError(err)
    }
    code = resp2.StatusCode
    status = classifyStatus(code, resp2.Status)
    io.Copy(io.Discard, resp2.Body)
    resp2.Body.Close()
    log.Printf("[LIVE] GET response for %s: %d %s", raw, code, status)
    return code, status
}

// classifyStatus provides a human-readable interpretation of HTTP status codes
func classifyStatus(code int, original string) string {
    switch {
    case code >= 200 && code < 300:
        return "OK"  // 2xx = success
    case code >= 300 && code < 400:
        return original  // 3xx = redirect (followed automatically)
    case code == 403:
        return "403 Forbidden"  // May be alive but blocked
    case code == 429:
        return "429 Rate Limited"  // Alive but throttled
    case code >= 400 && code < 500:
        return original  // 4xx = client error (likely dead)
    case code >= 500:
        return original  // 5xx = server error (dead/temporary)
    default:
        return original
    }
}

// classifyError provides human-readable error messages for network failures
func classifyError(err error) string {
    if err == nil {
        return "unknown"
    }
    errStr := err.Error()
    switch {
    case strings.Contains(errStr, "no such host"), strings.Contains(errStr, "DNS"):
        return "DNS lookup failed"
    case strings.Contains(errStr, "certificate"), strings.Contains(errStr, "tls"), strings.Contains(errStr, "TLS"):
        return "TLS/certificate error"
    case strings.Contains(errStr, "timeout"), strings.Contains(errStr, "deadline exceeded"):
        return "timeout"
    case strings.Contains(errStr, "connection refused"):
        return "connection refused"
    case strings.Contains(errStr, "connection reset"):
        return "connection reset"
    default:
        return "network error"
    }
}

// isArchiveURL detects if a URL is already an archive URL
func isArchiveURL(rawURL string) bool {
    lower := strings.ToLower(rawURL)
    archiveHosts := []string{
        "web.archive.org",           // Internet Archive Wayback Machine
        "archive.org/web/",          // Alternative Wayback path
        "archive.today",             // archive.today family
        "archive.is",
        "archive.ph",
        "archive.fo",
        "archive.li",
        "archive.md",
        "archive.vn",
        "webcitation.org",           // WebCite
        "perma.cc",                  // Perma.cc
        "archive-it.org",            // Archive-It
        "webarchive.org.uk",         // UK Web Archive
        "webarchive.nationalarchives.gov.uk", // UK National Archives
        "arquivo.pt",                // Portuguese Web Archive
        "webarchive.library.unt.edu", // UNT Web Archive
        "webarchive.loc.gov",        // Library of Congress
        "swap.stanford.edu",         // Stanford Web Archive Portal
        "vefsafn.is",                // Icelandic Web Archive
        "screenshots.com",           // Screenshots archive
    }

    for _, host := range archiveHosts {
        if strings.Contains(lower, host) {
            return true
        }
    }
    return false
}

func checkWayback(ctx context.Context, raw string) (bool, string, string) {
    // Wayback "available" v2 API
    v := url.Values{}
    v.Set("url", raw)
    // TODO: Investigate correct format for statuscodes parameter - comma-separated breaks API
    // The official IABot uses this, but our tests show it returns empty results
    // v.Set("statuscodes", "200,203,206")
    reqURL := "https://archive.org/wayback/available?" + v.Encode()
    ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
    defer cancel()

    log.Printf("[WAYBACK] Checking %s", raw)
    req, _ := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
    req.Header.Set("User-Agent", "IABot-Go/0.1 (+https://github.com/comaeclipse/IABot-Go)")
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        log.Printf("[WAYBACK] Request failed for %s: %v", raw, err)
        return false, "", "error: " + err.Error()
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        log.Printf("[WAYBACK] Non-OK status for %s: %d %s", raw, resp.StatusCode, resp.Status)
        return false, "", "HTTP " + resp.Status
    }

    b, err := io.ReadAll(resp.Body)
    if err != nil {
        log.Printf("[WAYBACK] Read error for %s: %v", raw, err)
        return false, "", "read error"
    }

    // Log the raw response for debugging
    log.Printf("[WAYBACK] Raw API response for %s: %s", raw, string(b))

    var wb struct {
        ArchivedSnapshots struct {
            Closest struct {
                Available bool   `json:"available"`
                URL       string `json:"url"`
                Timestamp string `json:"timestamp"`
                Status    string `json:"status"`
            } `json:"closest"`
        } `json:"archived_snapshots"`
    }
    if err := json.Unmarshal(b, &wb); err != nil {
        log.Printf("[WAYBACK] JSON decode error for %s: %v", raw, err)
        return false, "", "decode error: " + err.Error()
    }

    c := wb.ArchivedSnapshots.Closest
    log.Printf("[WAYBACK] Parsed response for %s: Available=%v, URL=%s, Status=%s, Timestamp=%s", raw, c.Available, c.URL, c.Status, c.Timestamp)

    if c.Available && c.URL != "" {
        // Validate timestamp (format: YYYYMMDDHHmmss)
        if !isValidArchiveTimestamp(c.Timestamp) {
            log.Printf("[WAYBACK] Invalid timestamp for %s: %s (rejected)", raw, c.Timestamp)
            return false, "", "invalid archive timestamp"
        }
        // Filter by status code - only accept good snapshots (200, 203, 206)
        // Do this server-side since the API parameter doesn't work as expected
        if c.Status != "200" && c.Status != "203" && c.Status != "206" {
            log.Printf("[WAYBACK] Bad snapshot status for %s: %s (rejected, only accepting 200/203/206)", raw, c.Status)
            return false, "", fmt.Sprintf("snapshot has bad status: %s", c.Status)
        }
        log.Printf("[WAYBACK] Found archive for %s: %s (status: %s)", raw, c.URL, c.Status)
        return true, c.URL, c.Status
    }
    log.Printf("[WAYBACK] No archive found for %s (Available=%v, URL empty=%v)", raw, c.Available, c.URL == "")
    return false, "", "not archived"
}

// isValidArchiveTimestamp validates Wayback Machine timestamps (format: YYYYMMDDHHmmss)
// Rejects timestamps before 1996-03-01 (when Wayback started) or in the future
func isValidArchiveTimestamp(timestamp string) bool {
    if len(timestamp) != 14 {
        return false  // Must be exactly 14 characters
    }

    // Parse timestamp: YYYYMMDDHHmmss
    t, err := time.Parse("20060102150405", timestamp)
    if err != nil {
        return false  // Invalid format
    }

    // Wayback Machine started on March 1, 1996
    waybackStart := time.Date(1996, 3, 1, 0, 0, 0, 0, time.UTC)
    if t.Before(waybackStart) {
        return false  // Too old
    }

    // Reject future timestamps (with 7 day buffer for timezone/indexing issues)
    // The Wayback API sometimes returns timestamps slightly ahead due to processing
    futureLimit := time.Now().UTC().Add(7 * 24 * time.Hour)
    if t.After(futureLimit) {
        return false  // In the future
    }

    return true
}
