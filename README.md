IABot-Go (skeleton)

This is a very minimal filesystem/architecture skeleton for an InternetArchiveBot-style project in Go.
It includes a single web interface page to prove the wiring. No real logic is implemented yet.

Layout

- cmd/iabot-web: tiny HTTP server that renders a single interface page
- internal/web: HTTP handler and template(s)

Run locally

1. Ensure you have Go 1.20+ installed
2. From this folder, run:
   - `go run ./cmd/iabot-web` (starts on http://localhost:8081)

Next steps (suggested)

- Add packages such as `api`, `parse`, `citemap`, `db`, `deadcheck`, and `rewrite`
- Wire basic configuration loading and health endpoints
- Add static assets and a richer UI

