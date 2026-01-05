# IABot-Go

A fast, reliable link checker and archival tool for Wikipedia articles, built in Go.

## Motivation

This project was born out of the reality that the [official Internet Archive Bot](https://meta.wikimedia.org/wiki/InternetArchiveBot) for Wikipedia is essentially **dead due to lack of development and excessive timeout errors**. Wikipedia editors need a working tool to:

- Check external links in articles for availability
- Identify which links have Wayback Machine archives
- Submit unarchived URLs to the Wayback Machine
- Cross-reference URLs with their citation numbers

IABot-Go provides a clean, performant alternative that actually works.

## Features

### Current Implementation

- **Citation Cross-Reference** - Parses Wikipedia wikitext to match URLs with citation numbers `[1]`, `[2]`, etc.
- **Dual View Modes** - Toggle between "By URL" (with refs) and "By Citation" (ordered by reference)
- **Live Link Checking** - Tests if external URLs are still accessible (200/403/404/timeout)
- **Wayback Archive Detection** - Queries Internet Archive's availability API
- **Save Page Now Integration** - Submit unarchived URLs directly to the Wayback Machine
- **Fast & Reliable** - Go's concurrency handles checks efficiently without timeouts

### How It Works

1. Enter a Wikipedia page title (e.g., "Albert Einstein")
2. IABot-Go fetches the page's wikitext via MediaWiki API
3. Parses `<ref>` tags to extract citations and URLs
4. Checks each URL for:
   - **Live status**: HEAD/GET request to test availability
   - **Archive status**: Wayback Machine availability API
5. Display results with citation numbers for easy cross-reference

## Run Locally

1. Ensure you have **Go 1.20+** installed
2. Clone this repository
3. From the project folder:
   ```bash
   go run ./cmd/iabot-web
   ```
4. Open http://localhost:8081 in your browser

## Usage

### Basic Link Checking

- Enter a Wikipedia page title
- View results in two modes:
  - **By URL**: Shows live/archive status with citation numbers
  - **By Citation**: Groups URLs by reference number

### Archive URLs (Save Page Now)

1. Get free API credentials from https://archive.org/account/s3.php
2. Click "Archive Now" next to unarchived URLs
3. Enter your access key and secret key (stored in browser session only)
4. IABot-Go submits the URL and polls for completion

## Project Structure

```
cmd/iabot-web/      - HTTP server entry point
api/
  index.go          - Main page handler, link checking, data structures
  parser.go         - Wikipedia wikitext citation parsing
  spn.go            - Save Page Now API client
  templates/        - HTML templates
```

## Credits

Built with frustration over the official IABot's reliability issues. Powered by:

- [MediaWiki API](https://www.mediawiki.org/wiki/API:Main_page)
- [Wayback Machine Availability API](https://archive.org/help/wayback_api.php)
- [Save Page Now (SPN) API](https://docs.archive.org/developers/tutorial-get-ia-credentials.html)
