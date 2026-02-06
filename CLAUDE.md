# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Gamwich is a self-hosted family household management web application ("digital refrigerator door") built in Go. It targets wall-mounted touchscreens and mobile companion use. The project is in early development — the module is defined but no source code exists yet.

- **Module:** `github.com/dukerupert/gamwich`
- **Go version:** 1.25.4
- **Database:** SQLite
- **Frontend stack (planned):** Go templates + HTMX + Alpine.js + Tailwind CSS + DaisyUI
- **Migrations:** goose
- **Real-time:** WebSocket push updates across connected screens
- **Deployment:** Single binary with embedded assets, targeting home servers / Raspberry Pi

## Build & Development Commands

No build tooling is configured yet. Standard Go commands apply:

```bash
go build ./...        # Build
go test ./...         # Run all tests
go test ./pkg/...     # Run tests in a specific package
go run .              # Run the application
```

## Architecture

The application is a monolithic Go HTTP server serving both API endpoints and rendered HTML pages. Key planned components:

- **Dashboard** — glanceable home screen (clock, weather, calendar summary, chores, grocery)
- **Family Calendar** — day/week/month views with CalDAV sync (Phase 2)
- **Chore Lists** — assignment, rotation, completion tracking, points system
- **Grocery/Shopping Lists** — categorized items, multiple lists
- **Notes/Message Board** — pinned household messages
- **Meal Planning** (Phase 2) — weekly scheduling with recipe links

## Design Principles

- Glanceable from 6 feet away, tap-first interactions
- Sub-100ms page transitions
- Offline-resilient, self-hosted (family data stays local)
- Simple over clever — no bloat

## Deployment

CI/CD is handled by GitHub Actions (`.github/workflows/deploy.yml`). The pipeline runs tests, builds a Docker image, pushes it to Docker Hub (`dukerupert/gamwich`), then SSHs into the VPS to pull and restart.

**Triggers:**
- Push to `main` — builds with `latest` and commit SHA tags, deploys automatically
- Push a `v*` tag — builds with semver tags (e.g. `1.2.3`, `1.2`), deploys automatically

**To release a new version:**

```bash
# 1. Tag the current commit with a semver version
git tag v1.2.3

# 2. Push the tag to trigger the CI/CD pipeline
git push origin v1.2.3
```

This will: run tests → build Docker image → push to Docker Hub (tagged `1.2.3`, `1.2`, and the commit SHA) → SSH deploy to VPS via `docker compose pull && docker compose up -d`.

**Required GitHub Secrets:**
- `DOCKERHUB_USERNAME` / `DOCKERHUB_TOKEN` — Docker Hub credentials
- `VPS_HOST` / `VPS_USER` / `VPS_SSH_KEY` / `VPS_DEPLOY_PATH` — VPS SSH and deploy path

## Key Documentation

- `dev/features.md` — comprehensive feature specification and data model
- `dev/implementation-plan.md` — implementation plan (to be filled)
