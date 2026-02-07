# Gamwich — Implementation Plan

> Detailed implementation steps for building the Gamwich family dashboard. Each milestone is a deployable increment. Tasks within each milestone are ordered for incremental development and testable commits.
>
> **Product model:** Free open-source core (self-hosted) with paid Cloud and Hosted tiers. See `dev/business-plan.md` for pricing, go-to-market, and revenue projections. See `dev/features.md` for the full feature spec.

---

## Phase 1: MVP — The Fridge Door Replacement ✅

The goal of Phase 1 is a working app that a family can actually use on a kitchen touchscreen to replace paper lists and fridge-door clutter. Every milestone in this phase ends with something usable.

**Status:** Complete

---

### Milestone 1.1: Project Scaffold & Family Members ✅

**Goal:** Go project structure, database, and the ability to manage family members. The foundation everything else builds on.

#### Tasks

1. **~~Initialize Go module and project structure~~** ✅
   - `go mod init gamwich`
   - Establish directory layout: `cmd/gamwich/`, `internal/`, `web/templates/`, `web/static/`, `db/migrations/`
   - Create `main.go` with basic HTTP server startup

2. **~~Set up SQLite database layer~~** ✅
   - Driver: `modernc.org/sqlite` (pure Go, no CGO)
   - Migration runner with embedded SQL files (`embed` package)
   - Initial migration: `family_members` table

3. **~~Create family member CRUD API~~** ✅
   - REST endpoints: Create, List, Update, Delete
   - Validation: name required, color format, unique name

4. **~~Build family member management page~~** ✅
   - Go template layout with DaisyUI (garden theme)
   - htmx-powered add/edit/delete forms
   - Color picker, drag-to-reorder, toast notifications

5. **~~Implement PIN selection for family members~~** ✅
   - Optional 4-digit PIN per member, bcrypt hashed
   - Touch-friendly number pad component

---

### Milestone 1.2: Dashboard Shell & Navigation ✅

**Goal:** The main touchscreen interface — a dashboard layout that will host all the widgets, plus navigation between sections.

#### Tasks

1. **~~Create the dashboard layout template~~** ✅
2. **~~Implement section routing with htmx~~** ✅
3. **~~Add weather widget~~** ✅
4. **~~Build the "Who am I?" quick-select~~** ✅

---

### Milestone 1.3: Calendar — Basic CRUD ✅

**Goal:** View and manage calendar events. Day and week views with create/edit/delete.

#### Tasks

1. **~~Create calendar database tables~~** ✅
2. **~~Build calendar event model and store~~** ✅ (10 unit tests)
3. **~~Build calendar event API~~** ✅
4. **~~Build day view~~** ✅
5. **~~Build week view~~** ✅
6. **~~Quick-add event form~~** ✅

---

### Milestone 1.4: Calendar — Recurring Events ✅

**Goal:** Support for repeating events (weekly soccer practice, monthly book club, birthdays).

#### Tasks

1. **~~Add recurrence to the data model~~** ✅ (RFC 5545 RRULE subset)
2. **~~Implement recurrence expansion logic~~** ✅ (`internal/recurrence/` package, 21 unit tests)
3. **~~Update event API to handle recurrence~~** ✅ (edit/delete "this event" vs "all events")
4. **~~Update UI for recurring events~~** ✅

---

### Milestone 1.5: Chore Lists ✅

**Goal:** Assign, track, and complete household chores.

#### Tasks

1. **~~Create chore database tables~~** ✅ (chore_areas, chores, chore_completions)
2. **~~Build chore CRUD API~~** ✅ (17 store tests)
3. **~~Implement chore status logic~~** ✅ (`internal/chore/status.go`, 14 unit tests)
4. **~~Build chore list page~~** ✅ (tabs: All/By Person/By Area)
5. **~~Build chore management page~~** ✅
6. **~~Add chore summary widget to dashboard~~** ✅

---

### Milestone 1.6: Grocery Lists ✅

**Goal:** Shared grocery list with categories, add/check-off, and auto-categorization.

#### Tasks

1. **~~Create grocery database tables~~** ✅ (11 seeded categories, default list)
2. **~~Build grocery item API~~** ✅
3. **~~Implement auto-categorization~~** ✅
4. **~~Build grocery list page~~** ✅
5. **~~Add grocery count widget to dashboard~~** ✅

---

### Milestone 1.7: Real-Time Sync with WebSockets ✅

**Goal:** When someone adds a grocery item from their phone, the kitchen screen updates instantly.

#### Tasks

1. **~~Set up WebSocket server~~** ✅ (hub pattern, JSON messages)
2. **~~Integrate WebSocket broadcasts into API handlers~~** ✅
3. **~~Implement client-side htmx refresh on WebSocket message~~** ✅

---

### Milestone 1.8: Responsive Mobile Companion ✅

**Goal:** The same app works well on a phone browser for on-the-go use.

#### Tasks

1. **~~Responsive layout audit and fixes~~** ✅ (375px–1920px)
2. **~~Touch interaction audit~~** ✅
3. **~~Add to Home Screen / PWA basics~~** ✅ (manifest.json, service worker)
4. **~~Network-aware behavior~~** ✅ (reconnection banner)

---

### Milestone 1.9: Touchscreen Kiosk Polish ✅

**Goal:** Optimize the experience for a dedicated kitchen touchscreen running in kiosk mode.

#### Tasks

1. **~~Idle / screensaver mode~~** ✅
2. **~~Night mode scheduling~~** ✅
3. **~~Kiosk deployment documentation~~** ✅
4. **~~Burn-in prevention~~** ✅

---

## Phase 2: Quality of Life

Phase 2 adds features that make the app stickier and more connected to the family's existing digital life. Some milestones are complete; the remainder are prioritized below.

---

### Milestone 2.1: Notes & Message Board ✅

**Goal:** Replace sticky notes on the fridge with pinned digital messages.

**Status:** Complete

#### Tasks

1. **~~Create notes database table~~** ✅ (priority enum: urgent/normal/fun)
2. **~~Build notes API~~** ✅ (CRUD + auto-expire cleanup)
3. **~~Build notes page~~** ✅ (card-based, color-coded by priority)
4. **~~Add pinned notes widget to dashboard~~** ✅

---

### Milestone 2.2: Rewards & Points System ✅

**Goal:** Optional gamification to motivate chore completion, especially for kids.

**Status:** Complete

#### Tasks

1. **~~Add points infrastructure~~** ✅ (rewards table, reward_redemptions)
2. **~~Build points tracking~~** ✅
3. **~~Build reward management~~** ✅ (settings page + redemption flow)
4. **~~Add points/rewards UI elements~~** ✅ (leaderboard widget)

---

### Milestone 2.3: Visual Polish & Theming ✅

**Goal:** DaisyUI theme system with curated themes and auto dark mode.

**Status:** Complete

#### Tasks

1. **~~Theme selection~~** ✅ (garden, forest, gamwich custom theme)
2. **~~Auto dark mode~~** ✅ (system preference + quiet hours)
3. **~~Animations and micro-interactions~~** ✅

---

### Milestone 2.4: Meal Planning

**Goal:** Plan meals for the week, display tonight's dinner on the dashboard, and push ingredients to the grocery list.

#### Tasks

1. **Create meal planning database tables**
   - Migration: `meals` table (`id`, `household_id`, `date`, `meal_type`, `title`, `recipe_url`, `notes`, `created_at`, `updated_at`)
   - Migration: `meal_ingredients` table (`id`, `meal_id`, `name`, `quantity`, `unit`, `category`)
   - `meal_type` enum: breakfast, lunch, dinner, snack
   - _Test: Migrations run_

2. **Build meal planning API**
   - `POST /api/meals` — create/assign a meal to a date
   - `GET /api/meals?start={date}&end={date}` — list meals in range
   - `PUT /api/meals/{id}` — update
   - `DELETE /api/meals/{id}` — remove from plan
   - `POST /api/meals/{id}/to-grocery` — push ingredients to grocery list
   - All endpoints scoped to household via auth context
   - _Test: CRUD + grocery push via curl_

3. **Build weekly meal planning page**
   - 7-column grid (like week calendar view), rows for meal types
   - Tap empty slot → add meal (title, optional recipe URL, optional notes)
   - Tap filled slot → edit, delete, or "add ingredients to grocery list"
   - _Test: Plan a week of dinners, push ingredients to grocery list_

4. **Build family favorites**
   - `saved_meals` table: title, recipe_url, notes, ingredients, use_count, household_id
   - "Save as favorite" option when adding a meal
   - Quick-add from favorites when planning
   - _Test: Save a meal, reuse it on another day_

5. **Add "What's for dinner?" widget to dashboard**
   - Prominent display of tonight's planned meal
   - Shows meal title and recipe link if available
   - Fallback: "No dinner planned — tap to add"
   - _Test: Plan tonight's dinner, verify it shows on dashboard_

---

### Milestone 2.5: CalDAV Integration

**Goal:** Bidirectional sync with Google Calendar, iCloud, or any CalDAV server.

#### Tasks

1. **Research and select Go CalDAV library**
   - Evaluate `emersion/go-webdav` or similar
   - Determine if acting as CalDAV client, server, or both
   - Document decision and limitations

2. **Build CalDAV client sync**
   - Settings page: add CalDAV account (URL, username, password/token)
   - Periodic sync job (configurable interval, default 15 minutes)
   - Map external events to Gamwich events, track sync state (etag/ctag)
   - Handle conflicts: external wins by default, option to flag for review

3. **Build outbound sync (Gamwich → CalDAV)**
   - Events created/modified in Gamwich push to the linked CalDAV calendar
   - Respect sync direction settings (one-way in, one-way out, bidirectional)

4. **Calendar source indicators in UI**
   - Visual badge on events showing origin (Gamwich, Google, iCloud, etc.)
   - External events may be read-only depending on sync settings

---

### Milestone 2.6: Smart Grocery Features

**Goal:** Make the grocery list smarter with frequent items and better categorization.

#### Tasks

1. **Build frequent items tracking**
   - Track item add frequency over time
   - `GET /api/grocery-lists/{list_id}/frequent` — return top 20 most-added items

2. **Build frequent items quick-add panel**
   - Row of chips/buttons above the grocery list showing frequent items
   - Tap to add with previous quantity/category

3. **Improve auto-categorization**
   - Expand the static item → category map
   - Learn from manual category overrides (store user corrections)

4. **Checked items archive / re-add**
   - After clearing checked items, store them in a "recently purchased" list
   - Easy re-add: "You bought these last time" panel

---

## Phase 3: Authentication & Multi-Tenancy ✅

**Status:** Complete — see `dev/auth-plan.md` for the detailed implementation plan.

This phase added the auth foundation required by all product tiers.

---

### Milestone 3.1: Database Migration & Auth Models ✅

**Status:** Complete

#### Tasks

1. **~~Auth migration (011_add_auth.sql)~~** ✅
   - New tables: `households`, `users`, `household_members`, `sessions`, `magic_links`
   - Added `household_id` FK to all existing tables (family_members, calendar_events, chore_areas, chores, grocery_categories, grocery_lists, notes, rewards, settings)
   - Rebuilt UNIQUE constraints as `UNIQUE(household_id, name)` for family_members and chore_areas
   - Default household backfill for existing data

2. **~~New models~~** ✅
   - `User`, `Household`, `HouseholdMember`, `Session`, `MagicLink`

---

### Milestone 3.2: Auth Infrastructure ✅

**Status:** Complete

#### Tasks

1. **~~Auth stores~~** ✅ — user, household, session, magic_link (all with tests)
2. **~~Email client~~** ✅ — Postmark integration (`internal/email/postmark.go`)
3. **~~Auth context~~** ✅ — `internal/auth/context.go` with `AuthContext{UserID, HouseholdID, Role}`
4. **~~Middleware~~** ✅ — `RequireAuth`, `RequireAdmin` with HTMX-aware redirects
5. **~~Rate limiting~~** ✅ — in-memory rate limiter for auth endpoints

---

### Milestone 3.3: Tenant-Scope All Stores & Handlers ✅

**Status:** Complete

#### Tasks

1. **~~Add householdID parameter to all store methods~~** ✅
2. **~~Extract householdID from auth context in all handlers~~** ✅
3. **~~Validate active user cookie against current household~~** ✅

---

### Milestone 3.4: Auth Routes, Templates & Polish ✅

**Status:** Complete

#### Tasks

1. **~~Auth handlers~~** ✅ — login, register, verify, invite, logout, household switching
2. **~~Auth templates~~** ✅ — login, register, check_email, households picker
3. **~~Route split~~** ✅ — public routes (login/register/verify) vs protected routes (all else)
4. **~~Layout updates~~** ✅ — user info + logout in header, invite section in settings
5. **~~Household-scoped WebSocket broadcasts~~** ✅
6. **~~Cleanup goroutine~~** ✅ — hourly expired session/magic link cleanup

---

## Phase 4: Cloud Services (Paid Tier)

The Cloud tier monetizes the natural desire for remote access and data safety. The app still runs on the user's hardware — the Cloud tier adds connectivity and peace of mind. Target price: ~$5/month ($50/year).

**Prerequisite:** Phase 3 (auth & multi-tenancy) ✅

---

### Milestone 4.1: Billing & License Key System ✅

**Goal:** Stripe integration for subscription management and a license key mechanism to activate paid features on self-hosted instances.

**Status:** Complete — standalone billing service (`cmd/billing/`) with Stripe checkout/webhooks, magic link auth, license key generation (`GW-XXXX-XXXX-XXXX-XXXX`), and account management. Gamwich app (`cmd/gamwich/`) gains a license client with background validation, caching, and 7-day offline grace period. 38 tests across billing stores and license client.

#### Tasks

1. **~~Design subscription data model~~** ✅
   - Billing service has its own SQLite database with `accounts`, `subscriptions`, `license_keys`, `sessions` tables
   - Full migration with indexes and update triggers

2. **~~Build Stripe integration~~** ✅
   - `internal/billing/stripe/stripe.go` — Stripe SDK v82 wrapper
   - Checkout session creation (monthly + annual Cloud pricing)
   - Webhook handler: `checkout.session.completed`, `invoice.paid`, `invoice.payment_failed`, `customer.subscription.updated`, `customer.subscription.deleted`
   - Webhook signature verification

3. **~~Build subscription management~~** ✅
   - `internal/billing/store/subscription.go` — full CRUD with tests
   - Account dashboard: plan status, billing portal link, license key display
   - Graceful degradation: license client caches status, features continue during grace period

4. **~~Build license key system~~** ✅
   - `internal/billing/handler/license.go` — `POST /api/license/validate` (rate-limited)
   - `internal/license/client.go` — Gamwich-side client with `HasFeature()`, `IsFreeTier()`, `SetKey()`
   - Background validation every 24h, 7-day offline grace period
   - 7 license client tests

5. **~~Build account management UI~~** ✅
   - Billing service: login, pricing, account dashboard templates (DaisyUI)
   - Gamwich settings: "Subscription" card with license key management
   - Dashboard: dismissable upgrade CTA for free-tier users
   - Deployment: `Dockerfile.billing`, `docker-compose.billing.yml`, GitHub Actions CI/CD

---

### Milestone 4.2: Remote Access Tunnel ✅

**Goal:** Access a self-hosted Gamwich instance from anywhere without port forwarding, VPN, or dynamic DNS.

**Status:** Complete

#### Tasks

1. **~~Cloudflare Tunnel integration~~** ✅
   - `internal/tunnel/tunnel.go` — Manager struct managing `cloudflared` subprocess
   - `internal/tunnel/logparser.go` — Parses cloudflared stderr for connection state
   - Token-based configuration: user enters Cloudflare tunnel token in settings
   - Subdomain extracted from cloudflared logs, displayed in settings UI

2. **~~Tunnel lifecycle management~~** ✅
   - Start tunnel on app startup if Cloud tier active and tunnel enabled
   - Graceful shutdown (context cancel → SIGTERM, 10s timeout)
   - Auto-reconnect with exponential backoff (1s→60s cap, max 10 failures)
   - `GET /health` public endpoint for tunnel health checks
   - Status indicator in settings: Connected/Connecting/Reconnecting/Error/Stopped/Disabled
   - 10 unit tests (`internal/tunnel/tunnel_test.go`)

3. **~~Tunnel configuration UI~~** ✅
   - Settings page "Remote Access" card with enable/disable toggle, token input, status badge
   - Status polling every 10s via htmx
   - Feature-gated: shows "Cloud Feature" upgrade prompt without tunnel license
   - WebSocket broadcast on state changes

4. **~~Security considerations~~** ✅
   - Rate limiter updated with `RealIP()` — prefers CF-Connecting-IP → X-Forwarded-For → RemoteAddr
   - Tunnel only exposes Gamwich HTTP server
   - Existing session auth applies to all tunnel traffic
   - `cloudflared` installed in Dockerfile (multi-arch: amd64/arm64)

---

### Milestone 4.3: Encrypted Offsite Backups ✅

**Goal:** Daily encrypted backup of the SQLite database to managed cloud storage. One-click restore from any snapshot.

#### Tasks

1. **Build backup agent** ✅
   - `internal/backup/backup.go` — Manager with scheduled backups, RunNow, Restore, Download, Cleanup
   - `internal/backup/crypto.go` — AES-256-GCM encryption with Argon2id key derivation
   - Flow: checkpoint SQLite WAL → copy database → encrypt with passphrase + salt → upload to S3-compatible storage
   - Schedule: daily (configurable hour), retention: 7/14/30/90 days
   - Passphrase cached in memory for scheduled backups after first manual entry

2. **Build backup API** ✅
   - `POST /partials/settings/backup/now` — trigger manual backup (passphrase required)
   - `GET /partials/settings/backup/history` — list available backup snapshots
   - `POST /partials/settings/backup/restore/{id}` — download, decrypt, validate integrity, replace database, exit for restart
   - `GET /partials/settings/backup/download/{id}` — stream encrypted backup file
   - Feature-gated via `licenseClient.HasFeature("backup")`

3. **Build backup settings UI** ✅
   - Settings card: passphrase setup, schedule toggle, backup hour (UTC), retention period
   - "Backup Now" with passphrase confirmation, polling status badge
   - History table with download/restore actions per backup
   - Change passphrase section with warning about existing backups
   - No-license → "Cloud Feature" upgrade prompt

4. **Server-side infrastructure** ✅
   - S3-compatible storage via AWS SDK Go v2 (AWS S3, Cloudflare R2, MinIO)
   - Per-household storage prefix (`{householdID}/{timestamp}.db.enc`)
   - Cleanup job: delete backups past retention window + remove S3 objects
   - Storage usage tracking per household (`TotalSizeByHousehold`)

---

### Milestone 4.4: Push Notifications ✅

**Goal:** Mobile push notifications for calendar reminders, chore due dates, and grocery list updates.

**Status:** Complete — Web Push (RFC 8030) with VAPID authentication via `webpush-go`. Per-user subscription management, notification preferences (calendar_reminder, chore_due, grocery_added), deduplication via `sent_notifications` table, 60-second background scheduler, service worker push/click handlers, and calendar event reminder UI. VAPID keys auto-generated on first startup if env vars are missing. 17 tests across push store and service.

#### Tasks

1. **~~Add push subscription data model~~** ✅
   - Migration `015_add_push_notifications.sql`: `push_subscriptions`, `notification_preferences`, `sent_notifications` tables + `reminder_minutes` column on `calendar_events`
   - Models: `internal/model/push.go` (PushSubscription, NotificationPreference)

2. **~~Build Web Push integration~~** ✅
   - `internal/push/push.go` — Service with Send, VAPIDPublicKey, GenerateVAPIDKeys (ECDSA P-256)
   - `internal/push/scheduler.go` — Background scheduler (60s tick): calendar reminders, chore due (hourly), grocery notifications (event-driven)
   - `internal/store/push.go` — Full CRUD + deduplication (15 store tests)

3. **~~Build notification triggers~~** ✅
   - Calendar: configurable reminder (15 min, 30 min, 1 hour, 1 day) with `<select>` in event create/edit forms
   - Chore due: daily summary sent once per hour (minute==0)
   - Grocery: fires when items added, excludes the adding user
   - Deduplication via UNIQUE(household_id, notification_type, reference_id, lead_time_minutes)

4. **~~Build push subscription UI~~** ✅
   - Service worker: push + notificationclick event handlers
   - Settings: Push Notifications card with Alpine.js subscribe/unsubscribe, preference toggles, test button, device list
   - Feature-gated via `licenseClient.HasFeature("push_notifications")`

---

## Phase 5: Hosted Tier

The Hosted tier targets non-technical families who want the Gamwich experience without managing hardware. We run the instance, they open a browser. Target price: ~$10/month ($100/year).

**Prerequisite:** Phase 4 (billing system) — shared Stripe integration

---

### Milestone 5.1: Multi-Tenant Hosted Infrastructure

**Goal:** Automated provisioning of managed Gamwich instances for paying customers.

#### Tasks

1. **Choose hosting platform**
   - Evaluate: Fly.io (per-app machines), Railway (container-based), bare VPS with Docker
   - Decision criteria: cost per idle instance, cold start time, operational simplicity
   - Target: <$3/month infrastructure cost per household at scale
   - Document decision and architecture

2. **Build provisioning system**
   - `internal/hosted/provisioner.go` — create/destroy/pause Gamwich instances
   - On sign-up: create container → assign subdomain → run migrations → seed defaults → ready in <30 seconds
   - Custom subdomain routing: `{family}.gamwich.app` → correct instance
   - Shared reverse proxy (Caddy or Traefik) routes subdomains to instances
   - _Test: Provision instance, access via subdomain, data is isolated_

3. **Build instance lifecycle management**
   - Auto-pause: if no requests for 7+ days, pause instance (reduce costs)
   - Auto-wake: first request to paused instance wakes it (<5 seconds)
   - Auto-update: roll new Gamwich versions to all hosted instances
   - Monitoring: health checks, resource usage per instance
   - _Test: Instance pauses after inactivity, wakes on request_

4. **Hosted sign-up flow**
   - Landing page: pricing, feature comparison, "Start free trial" CTA
   - Sign-up: email → choose household name → Stripe checkout → provision instance → redirect to `{family}.gamwich.app`
   - 14-day free trial (no credit card required for trial, required to continue)
   - _Test: Full sign-up flow end-to-end_

---

### Milestone 5.2: Hosted Tier Operations

**Goal:** Operational tooling for running a fleet of hosted Gamwich instances.

#### Tasks

1. **Admin dashboard**
   - Internal tool (not customer-facing): list all hosted instances, status, resource usage
   - Actions: pause/wake/delete instance, view logs, impersonate for support
   - _Test: Admin can view and manage instances_

2. **Automated backups for hosted instances**
   - All hosted instances get daily backups automatically (no Cloud tier license needed)
   - Same backup infrastructure as Cloud tier, just always-on
   - Customer can download their data at any time (data portability)

3. **Scale considerations**
   - At 1,000+ hosted households: evaluate shared PostgreSQL backend for the hosted platform
   - Self-hosted product stays on SQLite regardless
   - Document the threshold and migration path

---

## Phase 6: Extended Features

Features that add delight and depth for long-term daily use. These can be implemented in any order based on community demand.

---

### Milestone 6.1: Chore Rotation & Automation

**Goal:** Chores automatically rotate between family members on a schedule.

#### Tasks

1. **Add rotation support to data model**
   - Add `rotation_members` (JSON array of family_member_ids) and `rotation_interval` to `chores` table
   - Add `current_rotation_index` to track position

2. **Implement rotation logic**
   - When a rotating chore's period elapses, advance the assignee to the next member
   - Handle edge cases: member removed, member skipped

3. **Rotation UI**
   - Chore form: toggle "rotate between" → select members and interval
   - Chore list: "Your turn" badge, upcoming rotation preview

---

### Milestone 6.2: Multi-List Support

**Goal:** Separate grocery lists for different stores and purposes.

#### Tasks

1. **Enable multiple grocery lists**
   - List management: create, rename, reorder, delete lists
   - Each list has its own items and categories, default list configurable

2. **Update grocery UI for multi-list**
   - Tab bar or dropdown to switch between lists
   - Dashboard widget shows default list count with indicators for other active lists

---

### Milestone 6.3: Import & Integrations

**Goal:** Reduce manual data entry by importing from external sources.

#### Tasks

1. **School calendar import**
   - Import `.ics` file upload, parse iCal format, create events in Gamwich

2. **Grocery list sharing**
   - Shareable read-only link for a grocery list (no auth required on shared link)

3. **Data export**
   - Export all data as JSON backup
   - Export grocery list as plain text
   - Export calendar as .ics

---

### Milestone 6.4: Kiosk Deployment Tooling

**Goal:** Make it dead simple to set up a new Gamwich kitchen screen.

#### Tasks

1. **Single-binary distribution**
   - Embed all static assets, templates, and migrations into the Go binary
   - `gamwich serve` starts everything with sane defaults
   - `gamwich setup` runs interactive first-time configuration

2. **Docker image**
   - Dockerfile: minimal base image, single binary, SQLite volume mount
   - `docker-compose.yml` with volume for DB persistence

3. **Raspberry Pi kiosk installer script**
   - Bash script: installs Chromium, configures kiosk mode, sets up systemd service, auto-start

4. **Local backup/restore tooling**
   - Configurable backup schedule (daily default)
   - SQLite `.backup` to configurable path, retain N backups with cleanup
   - Settings page: backup now, restore from backup, download backup

---

### Milestone 6.5: Family Photo Idle Screen

**Goal:** Idle mode cycles through family photos with a clock overlay.

#### Tasks

1. **Photo upload**
   - Configure a photo directory or upload photos via settings page
   - Store in `web/static/photos/` or a configurable path

2. **Slideshow idle mode**
   - Idle mode transitions between photos with clock/date overlay
   - Configurable transition timing and shuffle/sequential order

---

### Milestone 6.6: Grocery Suggestions (Stretch)

**Goal:** Proactive reminders based on purchase history patterns.

#### Tasks

1. **Purchase history analysis**
   - Track when items are added and cleared, calculate average purchase frequency

2. **Suggestion engine**
   - "It's been 2 weeks since you bought eggs — add to list?"
   - Shown as dismissable cards, configurable sensitivity

---

## Appendix: Cross-Cutting Concerns

These apply throughout all phases and should be addressed continuously.

### Testing Strategy
- **Unit tests:** Go — domain logic, recurrence expansion, date calculations, auto-categorization, auth/session management
- **Integration tests:** API endpoints with test database, cross-tenant isolation verification
- **Manual testing:** Touchscreen interaction on actual device at each milestone
- **Browser testing:** Chrome (kiosk), Safari (iOS companion), Firefox

### Configuration
- **Environment-based config:** `GAMWICH_DB_PATH`, `GAMWICH_PORT`, `GAMWICH_TIMEZONE`, `GAMWICH_POSTMARK_TOKEN`, `GAMWICH_BASE_URL`, `GAMWICH_FROM_EMAIL`, `GAMWICH_STRIPE_KEY`, `GAMWICH_STRIPE_WEBHOOK_SECRET`, `GAMWICH_VAPID_PUBLIC_KEY`, `GAMWICH_VAPID_PRIVATE_KEY`
- **Settings page in UI:** weather location, quiet hours, default list, theme, sync intervals, notification preferences, backup schedule, tunnel status
- **Config file:** TOML or YAML for initial setup, overridable by env vars

### Security
- **Authentication:** Magic link passwordless auth (Postmark), session-based with HttpOnly cookies
- **Multi-tenancy:** All data queries scoped to household via auth context — cross-tenant leakage is a critical bug
- **Rate limiting:** In-memory rate limiter on auth endpoints (3 per email / 15 min, 10 per IP / 15 min)
- **HTMX-aware auth:** Expired sessions return `HX-Redirect` header instead of HTTP redirect
- **Tunnel security:** Remote access uses same session auth, tunnel only exposes Gamwich HTTP server
- **Backup encryption:** AES-256-GCM with user-held key, server never stores plaintext backups
- **Webhook verification:** Stripe webhook signatures verified before processing
- **HTTPS:** Via reverse proxy (Caddy recommended) or built-in; required for remote access

### Performance Targets
- **Page load:** <200ms on Raspberry Pi 4
- **WebSocket latency:** <100ms for real-time updates
- **Database:** SQLite handles household scale trivially — optimize only if measured
- **Frontend:** Minimal JS, htmx keeps payload small, Alpine.js for interactivity without framework weight, DaisyUI for consistent components without custom CSS overhead
- **Tunnel latency:** <50ms additional latency via Cloudflare edge
- **Hosted provisioning:** <30 seconds from sign-up to usable instance
