package handler

import (
	"regexp"
	"strings"
)

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
