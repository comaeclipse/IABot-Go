package handler

import (
    "context"
    "embed"
    "encoding/json"
    "html/template"
    "io"
    "net/http"
    "net/url"
    "sort"
    "strings"
    "time"
)

//go:embed templates/index.html
var tmplFS embed.FS

type pageData struct {
    Title   string
    Message string
    Query   string
    Results []linkResult
    Error   string
}

type linkResult struct {
    URL           string
    LiveCode      int
    LiveStatus    string
    Archived      bool
    ArchiveURL    string
    ArchiveStatus string
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
        if q != "" {
            data.Query = q
            results, err := scanPage(r.Context(), q)
            if err != nil {
                data.Error = err.Error()
            } else {
                data.Results = results
            }
        }
    }

    _ = t.Execute(w, data)
}

func scanPage(ctx context.Context, title string) ([]linkResult, error) {
    // Fetch external links via MediaWiki API (parse.externallinks)
    api := "https://en.wikipedia.org/w/api.php"
    v := url.Values{}
    v.Set("action", "parse")
    v.Set("page", title)
    v.Set("prop", "externallinks")
    v.Set("format", "json")
    reqURL := api + "?" + v.Encode()

    ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()

    req, _ := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    body, _ := io.ReadAll(resp.Body)

    // minimal JSON decode for externallinks
    var parsed struct {
        Parse struct {
            ExternalLinks []string `json:"externallinks"`
        } `json:"parse"`
        Error any `json:"error"`
    }
    if err := json.Unmarshal(body, &parsed); err != nil {
        return nil, err
    }
    links := parsed.Parse.ExternalLinks
    // De-duplicate and trim count to avoid long runs
    uniq := make(map[string]struct{})
    out := make([]string, 0, len(links))
    for _, u := range links {
        u = strings.TrimSpace(u)
        if u == "" {
            continue
        }
        if _, ok := uniq[u]; ok {
            continue
        }
        uniq[u] = struct{}{}
        out = append(out, u)
    }
    sort.Strings(out)
    if len(out) > 50 {
        out = out[:50]
    }

    results := make([]linkResult, 0, len(out))
    for _, u := range out {
        lr := linkResult{URL: u}
        code, status := checkLive(ctx, u)
        lr.LiveCode = code
        lr.LiveStatus = status
        arch, aurl, astatus := checkWayback(ctx, u)
        lr.Archived = arch
        lr.ArchiveURL = aurl
        lr.ArchiveStatus = astatus
        results = append(results, lr)
    }
    return results, nil
}

func checkLive(ctx context.Context, raw string) (int, string) {
    // Try HEAD then fallback to GET if HEAD returns 405 or fails
    status := "unknown"
    code := 0
    client := &http.Client{Timeout: 8 * time.Second}
    // HEAD
    req, err := http.NewRequestWithContext(ctx, http.MethodHead, raw, nil)
    if err == nil {
        if resp, err := client.Do(req); err == nil {
            code = resp.StatusCode
            status = resp.Status
            resp.Body.Close()
            if code != http.StatusMethodNotAllowed && code != http.StatusNotImplemented {
                return code, status
            }
        }
    }
    // GET with small range
    req2, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
    if err != nil {
        return code, status
    }
    req2.Header.Set("Range", "bytes=0-0")
    if resp, err := client.Do(req2); err == nil {
        code = resp.StatusCode
        status = resp.Status
        io.Copy(io.Discard, resp.Body)
        resp.Body.Close()
    }
    return code, status
}

func checkWayback(ctx context.Context, raw string) (bool, string, string) {
    // Wayback “available” v2 API
    v := url.Values{}
    v.Set("url", raw)
    reqURL := "https://archive.org/wayback/available?" + v.Encode()
    ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
    defer cancel()
    req, _ := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return false, "", "request error"
    }
    defer resp.Body.Close()
    b, _ := io.ReadAll(resp.Body)
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
        return false, "", "decode error"
    }
    c := wb.ArchivedSnapshots.Closest
    if c.Available && c.URL != "" {
        return true, c.URL, c.Status
    }
    return false, "", "none"
}

