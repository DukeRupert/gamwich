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

## Key Documentation

- `dev/features.md` — comprehensive feature specification and data model
- `dev/implementation-plan.md` — implementation plan (to be filled)
