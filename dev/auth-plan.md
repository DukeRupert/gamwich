# Authentication & Multi-Tenancy Plan

## Context

Gamwich currently has no authentication — all routes are public, and the active user is tracked via a client-side cookie (`gamwich_active_user`) storing a family member ID. This was intentional for trusted LAN kiosk use, but the app now needs:

1. **Multi-tenant support** — multiple households sharing one Gamwich instance with isolated data
2. **Secure remote access** — protect the app when accessed over the internet
3. **Passwordless auth** — magic link emails via Postmark

Auth accounts are **separate from family members** — parents/adults have login accounts, kids remain lightweight profiles. All devices (including kiosks) must authenticate.

---

## Phase 1: Database Migration & Models

### Migration: `internal/database/migrations/011_add_auth.sql`

**New tables:**

| Table | Key Columns |
|-------|------------|
| `households` | id, name, created_at, updated_at |
| `users` | id, email (UNIQUE), name, created_at, updated_at |
| `household_members` | id, household_id FK, user_id FK, role (admin/member), UNIQUE(household_id, user_id) |
| `sessions` | id, token (UNIQUE), user_id FK, household_id FK, expires_at |
| `magic_links` | id, token (UNIQUE), email, purpose (login/register/invite), household_id FK (nullable), expires_at, used_at |

**Schema changes to existing tables:**
- Add `household_id INTEGER REFERENCES households(id) ON DELETE CASCADE` to: `family_members`, `calendar_events`, `chore_areas`, `chores`, `grocery_categories`, `grocery_lists`, `notes`, `rewards`
- Recreate `settings` table with `id INTEGER PRIMARY KEY, household_id FK, key TEXT, value TEXT, UNIQUE(household_id, key)` (current PK is just `key TEXT`)
- Add indexes on `household_id` for all modified tables

**Data backfill:**
- Insert default household ("My Household", id=1)
- UPDATE all root tables SET household_id = 1
- Migrate settings rows into new table with household_id = 1

### New model files
- `internal/model/user.go` — User{ID, Email, Name, CreatedAt, UpdatedAt}
- `internal/model/household.go` — Household{ID, Name, ...}, HouseholdMember{ID, HouseholdID, UserID, Role, ...}
- `internal/model/session.go` — Session{ID, Token, UserID, HouseholdID, ExpiresAt, ...}, MagicLink{ID, Token, Email, Purpose, HouseholdID, ExpiresAt, UsedAt, ...}

---

## Phase 2: Auth Infrastructure

### New stores
- `internal/store/user.go` — Create, GetByID, GetByEmail, Update, Delete
- `internal/store/household.go` — Create, GetByID, Update, Delete, AddMember, RemoveMember, GetMember, ListMembers, ListHouseholdsForUser, UpdateMemberRole, **SeedDefaults** (inserts chore areas, grocery categories, default list, default settings for a new household)
- `internal/store/session.go` — Create (generates 32-byte random token, 90-day expiry), GetByToken (returns nil if expired), Delete, DeleteExpired, DeleteByUserID
- `internal/store/magic_link.go` — Create (generates 32-byte random token, 15-min expiry), GetByToken (nil if expired/used), MarkUsed, DeleteExpired

### Email client: `internal/email/postmark.go`
- Simple HTTP POST to `https://api.postmarkapp.com/email`
- Header: `X-Postmark-Server-Token`
- Method: `SendMagicLink(toEmail, token, purpose, householdName string) error`
- Constructs URL from `GAMWICH_BASE_URL` + `/auth/verify?token=xxx` (or `/invite/accept?token=xxx` for invites)
- Accepts `httpClient` override for testing (same pattern as weather service)
- Nil-safe: if no Postmark token configured, returns descriptive error

### Auth context: `internal/auth/context.go`
- `AuthContext{UserID, HouseholdID, Role, SessionID}` stored in request context
- Helper functions: `WithAuth()`, `FromContext()`, `HouseholdID()`, `UserID()`, `IsAdmin()`

### Middleware: `internal/middleware/auth.go`
- `RequireAuth(next http.Handler) http.Handler` — reads `gamwich_session` cookie, looks up session, verifies not expired, gets member role, populates `AuthContext` in request context
- **HTMX-aware redirects**: if `HX-Request: true` header present, returns `HX-Redirect: /login` (HTMX can't follow HTTP 303 redirects in swap context)
- `RequireAdmin(next http.Handler)` — checks `Role == "admin"`, returns 403 otherwise

### Auth handlers: `internal/handler/auth.go`
- `GET /login` — render login page (standalone, not using layout.html)
- `POST /login` — parse email, create magic link (purpose=login), send via Postmark, show "check your email" (always show success to prevent email enumeration)
- `GET /register` — render registration page (email + household name)
- `POST /register` — create household, create user, add user as admin, send magic link (purpose=register)
- `GET /auth/verify?token=xxx` — verify magic link, find user, create session, set `gamwich_session` cookie (HttpOnly, SameSite=Lax, 90 days), redirect to `/`
- `GET /invite/accept?token=xxx` — verify invite link, create user if needed, add to household, create session, redirect
- `POST /invite` — admin-only, parse email, create magic link (purpose=invite, household_id from context), send email
- `POST /logout` — delete session, clear cookie, redirect to `/login`

### Config changes: `cmd/gamwich/main.go`
- New env vars: `GAMWICH_POSTMARK_TOKEN`, `GAMWICH_BASE_URL`, `GAMWICH_FROM_EMAIL`
- Create email client, new stores
- Background goroutine: hourly cleanup of expired sessions and magic links
- Weather settings bootstrapping: remove household-specific settings read at startup (weather becomes per-request in Phase 3 or a follow-up)

---

## Phase 3: Tenant-Scope All Existing Stores & Handlers

**Pattern**: Every store method that reads/writes household data gains `householdID int64` as first parameter. Every handler extracts `householdID` from `auth.FromContext(r.Context())`.

### Stores to modify (add `householdID` param to all methods)
- `internal/store/family_member.go` — List, GetByID, Create, Update, Delete, SetPIN, ClearPIN, GetPINHash, NameExists, UpdateSortOrder
- `internal/store/calendar_event.go` — CreateWithRecurrence, GetByID, ListByDateRange, ListRecurring, ListExceptions, Update, Delete, CreateException, etc.
- `internal/store/chore.go` — area CRUD, chore CRUD, ListByAssignee, ListByArea, completion CRUD
- `internal/store/grocery.go` — category list, list/item CRUD, toggle, clear checked, count unchecked
- `internal/store/note.go` — CRUD, ListPinned, TogglePinned, DeleteExpired
- `internal/store/reward.go` — CRUD, ListActive, Redeem, ListRedemptionsByMember, PointBalance, GetAllPointBalances
- `internal/store/settings.go` — Get, Set, GetAll, GetKioskSettings, GetWeatherSettings, GetThemeSettings (also change INSERT OR REPLACE to `INSERT ... ON CONFLICT(household_id, key) DO UPDATE`)

### Handlers to modify (extract `householdID` from auth context, pass to store)
- `internal/handler/family_member.go`
- `internal/handler/calendar_event.go`
- `internal/handler/chore.go`
- `internal/handler/grocery.go`
- `internal/handler/note.go`
- `internal/handler/reward.go`
- `internal/handler/settings.go`
- `internal/handler/template.go` — largest file (3,400+ lines), every method needs updating

### Active user cookie validation
`activeUserFromCookie()` in template.go must verify the family member belongs to the current session's household (prevents cross-tenant leakage if a user switches households).

---

## Phase 4: Server Wiring, Routes & Templates

### Route split in `internal/server/server.go`
**Public routes** (registered directly on outer mux):
- `GET /login`, `POST /login`, `GET /register`, `POST /register`
- `GET /auth/verify`, `GET /invite/accept`
- `GET /static/*`

**Protected routes** (wrapped with `RequireAuth` middleware):
- All existing routes (dashboard, API, partials, WebSocket)
- `POST /logout`, `POST /invite`

Go 1.22+ ServeMux matches most-specific first, so `/login` matches before the catch-all `/` on the protected mux.

### New template files
- `web/templates/login.html` — standalone page (not layout.html), email input form, link to register
- `web/templates/register.html` — standalone page, email + household name
- `web/templates/check_email.html` — "Check your email" confirmation page

### Layout changes: `web/templates/layout.html`
- Add user email / household name display + logout button to header

### Settings page: `web/templates/settings.html`
- Add "Invite Member" section (admin-only) with email input form

---

## Phase 5: WebSocket Scoping & Polish

### Household-scoped broadcasts
- `internal/websocket/client.go` — add `HouseholdID int64` field to Client
- `internal/websocket/hub.go` — add `BroadcastToHousehold(householdID int64, msg Message)` that filters clients by household
- `internal/websocket/handler.go` — extract household from auth context during WebSocket upgrade
- All handler `broadcast()` helpers change to pass `householdID`

### Edge cases
- **UNIQUE constraints**: `family_members.name` and `chore_areas.name` have global UNIQUE. Enforce per-household uniqueness in application layer for now; table rebuild deferred to future migration.
- **Weather service**: Currently singleton with single lat/lon cache. For Phase 1, keep as-is (all households share weather config from env vars). Per-household weather is a follow-up task.
- **WebSocket + expired session**: If a session expires while WebSocket is connected, the connection stays alive. Acceptable — next HTTP request triggers re-auth.

---

## Files Summary

### New files (15)
| File | Purpose |
|------|---------|
| `internal/database/migrations/011_add_auth.sql` | Schema changes |
| `internal/model/user.go` | User model |
| `internal/model/household.go` | Household + HouseholdMember models |
| `internal/model/session.go` | Session + MagicLink models |
| `internal/store/user.go` | User CRUD |
| `internal/store/household.go` | Household CRUD + membership + seed defaults |
| `internal/store/session.go` | Session management |
| `internal/store/magic_link.go` | Magic link lifecycle |
| `internal/email/postmark.go` | Postmark API client |
| `internal/auth/context.go` | Auth context helpers |
| `internal/middleware/auth.go` | Auth + admin middleware |
| `internal/handler/auth.go` | Login, register, verify, invite, logout |
| `web/templates/login.html` | Login page |
| `web/templates/register.html` | Registration page |
| `web/templates/check_email.html` | Email sent confirmation |

### Modified files (17+)
| File | Changes |
|------|---------|
| `internal/store/family_member.go` | Add householdID param |
| `internal/store/calendar_event.go` | Add householdID param |
| `internal/store/chore.go` | Add householdID param |
| `internal/store/grocery.go` | Add householdID param |
| `internal/store/note.go` | Add householdID param |
| `internal/store/reward.go` | Add householdID param |
| `internal/store/settings.go` | Add householdID param, upsert rewrite |
| `internal/handler/family_member.go` | Extract householdID from context |
| `internal/handler/calendar_event.go` | Extract householdID from context |
| `internal/handler/chore.go` | Extract householdID from context |
| `internal/handler/grocery.go` | Extract householdID from context |
| `internal/handler/note.go` | Extract householdID from context |
| `internal/handler/reward.go` | Extract householdID from context |
| `internal/handler/settings.go` | Extract householdID from context |
| `internal/handler/template.go` | Extract householdID, auth data to templates |
| `internal/server/server.go` | New fields, route split, middleware |
| `internal/websocket/hub.go` | BroadcastToHousehold |
| `internal/websocket/client.go` | HouseholdID field |
| `internal/websocket/handler.go` | Auth context extraction |
| `web/templates/layout.html` | Logout button, user info |
| `web/templates/settings.html` | Invite section |
| `cmd/gamwich/main.go` | New config, email client, cleanup goroutine |
| `docker-compose.yml` | New env vars |

---

## Verification

1. `go test ./...` — all existing tests updated with householdID, new store/middleware/email tests pass
2. New tests verify cross-tenant isolation: data from household A never appears in household B queries
3. Manual flow: register -> check email -> click magic link -> session created -> dashboard loads -> invite another user -> they accept -> both see same household data
4. HTMX test: expired session on HTMX request returns `HX-Redirect: /login` (not HTML swap)
5. WebSocket test: broadcasts only reach clients in the same household

## Implementation Order
Phase 1 -> Phase 2 -> Phase 3 -> Phase 4 -> Phase 5 (strictly sequential, each builds on the last)
