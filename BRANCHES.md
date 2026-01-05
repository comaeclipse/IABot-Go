# Branch Structure

This repository has multiple branches for different deployment targets:

## `master`
- **Purpose**: Main development branch
- **Status**: Active development
- All new features are developed here first

## `render`
- **Purpose**: Traditional server deployment (Render, Railway, Fly.io, Docker)
- **Status**: Production-ready
- Runs as a long-lived Go web server on port 8081
- Suitable for any platform that supports traditional web servers
- **Deploy**: `go run ./cmd/iabot-web` or build with `go build`

## `vercel`
- **Purpose**: Vercel serverless functions
- **Status**: Work in progress
- Requires restructuring to work with Vercel's serverless model
- Each `api/*.go` file becomes a separate function endpoint

## Recommended Deployment

For most use cases, use the **`render`** branch:
- Works on Render.com, Railway.app, Fly.io
- Can be dockerized
- Runs locally for development
- Full feature support

The Vercel branch is experimental and may have limitations due to serverless constraints.
