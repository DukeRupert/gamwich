# Authentication & Multi-Tenancy Plan

## Context

Gamwich currently has no authentication — all routes are public, and the active user is tracked via a client-side cookie (`gamwich_active_user`) storing a family member ID. This was intentional for trusted LAN kiosk use, but the app now needs authentication to support the product model described in `dev/business-plan.md`:

1. **Multi-tenant support** — multiple households sharing one Gamwich instance with isolated data (required for the hosted tier, useful for self-hosted users sharing an instance)
2. **Secure remote access** — protect the app when accessed over the internet (required for both self-hosted remote use and the cloud tier's tunnel relay)
3. **Passwordless auth** — one-time code emails via Postmark (simple for families, no passwords to manage)

Auth accounts are **separate from family members** — parents/adults have login accounts, kids remain lightweight profiles. All devices (including kiosks) must authenticate.

### Product Tier Implications

This plan covers the auth foundation needed by **all tiers** (free, cloud, hosted). Cloud-tier features (tunnel relay, encrypted backups, push notifications) and hosted-tier infrastructure are out of scope here — they build on top of the auth system defined below. See `dev/business-plan.md` for those details.

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
| `magic_links` | id, token (6-digit code), email, purpose (login/register/invite), household_id FK (nullable), expires_at, used_at, attempts |

**Schema changes to existing tables:**
- Add `household_id INTEGER REFERENCES households(id) ON DELETE CASCADE` to: `family_members`, `calendar_events`, `chore_areas`, `chores`, `grocery_categories`, `grocery_lists`, `notes`, `rewards`
- Recreate `settings` table with `id INTEGER PRIMARY KEY, household_id FK, key TEXT, value TEXT, UNIQUE(household_id, key)` (current PK is just `key TEXT`)
- Add indexes on `household_id` for all modified tables
- **Fix UNIQUE constraints**: `family_members` and `chore_areas` currently have global `UNIQUE(name)`. These must be rebuilt as `UNIQUE(household_id, name)` — otherwise two households cannot have a family member with the same name (e.g. both have "Mom"). SQLite requires table rebuild (CREATE new → INSERT SELECT → DROP old → RENAME) since it doesn't support DROP CONSTRAINT.

**Migration strategy for adding household_id (wrap in `-- +goose StatementBegin/End`):**
SQLite doesn't support `ALTER TABLE ADD COLUMN ... NOT NULL` on tables with existing rows unless a default is provided. Use this approach for each table:
1. Insert default household ("My Household", id=1) first
2. Add column as `household_id INTEGER DEFAULT 1 REFERENCES households(id) ON DELETE CASCADE`
3. For tables needing UNIQUE constraint changes (`family_members`, `chore_areas`), use the full table-rebuild approach: CREATE new table with `UNIQUE(household_id, name)` → INSERT SELECT with household_id=1 → DROP old → RENAME
4. For `settings`: CREATE `settings_new` with new schema → INSERT SELECT with household_id=1 → DROP `settings` → RENAME

**Data backfill:**
- Insert default household ("My Household", id=1)
- Column additions with DEFAULT 1 handle backfill automatically
- Table rebuilds copy existing data with household_id = 1
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
- `internal/store/magic_link.go` — Create (generates 6-digit numeric code, 15-min expiry, invalidates previous codes for same email), GetByEmailAndCode, GetLatestByEmail, IncrementAttempts, MarkUsed, DeleteExpired

### Email client: `internal/email/postmark.go`
- Simple HTTP POST to `https://api.postmarkapp.com/email`
- Header: `X-Postmark-Server-Token`
- Method: `SendAuthCode(toEmail, code, purpose, householdName string) error`
- For login/register: email body contains only the 6-digit code
- For invite: email body contains both a navigation URL (`{baseURL}/invite/accept?email={encoded}`) and the 6-digit code
- Accepts `httpClient` override for testing (same pattern as weather service)
- Nil-safe: if no Postmark token configured, returns descriptive error

### Auth context: `internal/auth/context.go`
- `AuthContext{UserID, HouseholdID, Role, SessionID}` stored in request context
- Helper functions: `WithAuth()`, `FromContext()`, `HouseholdID()`, `UserID()`, `IsAdmin()`

### Middleware: `internal/middleware/auth.go`
- `RequireAuth(next http.Handler) http.Handler` — reads `gamwich_session` cookie, looks up session, verifies not expired, gets member role, populates `AuthContext` in request context
- **HTMX-aware redirects**: if `HX-Request: true` header present, returns `HX-Redirect: /login` (HTMX can't follow HTTP 303 redirects in swap context)
- `RequireAdmin(next http.Handler)` — checks `Role == "admin"`, returns 403 otherwise

### Rate limiting: `internal/middleware/ratelimit.go`
- In-memory rate limiter for auth endpoints (no external dependency)
- Keyed by email address (for login/register) and by IP (for all auth endpoints)
- Limits: 10 auth requests per IP per minute
- Returns 429 Too Many Requests with Retry-After header
- Simple `sync.Map` with TTL cleanup — no need for Redis at this scale

### Auth handlers: `internal/handler/auth.go`
- `GET /login` — render login page (standalone, not using layout.html)
- `POST /login` — rate-limited, parse email, create auth code (purpose=login), send via Postmark, show code entry form (always show form to prevent email enumeration)
- `GET /register` — render registration page (email + household name)
- `POST /register` — rate-limited, create household, create user, add user as admin, send auth code (purpose=register)
- `POST /auth/verify` — rate-limited, validate email + 6-digit code (max 5 attempts per code), create session, set `gamwich_session` cookie (HttpOnly, SameSite=Lax, Secure if HTTPS, 90 days), redirect to `/`
- `GET /invite/accept?email=xxx` — show code entry form with email pre-filled
- `POST /invite/accept` — rate-limited, validate email + 6-digit code, create user if needed, add to household, create session, redirect
- `POST /invite` — admin-only, rate-limited, parse email, create auth code (purpose=invite, household_id from context), send email
- `POST /logout` — delete session, clear cookie, redirect to `/login`

### Household switching (multi-household users)
A user can belong to multiple households (e.g., invited to a neighbor's instance). The session stores a single `household_id`, so switching requires:
- `GET /households` — list households the user belongs to (only shown if user belongs to >1)
- `POST /switch-household` — update session's `household_id`, clear `gamwich_active_user` cookie (stale family member ID from previous household), redirect to `/`
- After login, if user belongs to multiple households, redirect to `/households` picker instead of `/`
- Layout header shows current household name as a link to the picker (when user has multiple)

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
- `POST /auth/verify`, `GET /invite/accept`, `POST /invite/accept`
- `GET /static/*`

**Protected routes** (wrapped with `RequireAuth` middleware):
- All existing routes (dashboard, API, partials, WebSocket)
- `POST /logout`, `POST /invite`

Go 1.22+ ServeMux matches most-specific first, so `/login` matches before the catch-all `/` on the protected mux.

### New template files
- `web/templates/login.html` — standalone page (not layout.html), email input form, link to register
- `web/templates/register.html` — standalone page, email + household name
- `web/templates/check_email.html` — "Check your email" confirmation page
- `web/templates/households.html` — household picker for users belonging to multiple households

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
- **UNIQUE constraints**: `family_members.name` and `chore_areas.name` global UNIQUE constraints are rebuilt as `UNIQUE(household_id, name)` in migration 011 (see Phase 1 migration strategy). This is required — without it, two households cannot share common names like "Mom" or "Kitchen".
- **Weather service**: Currently singleton with single lat/lon cache. For initial rollout, keep as-is (all households share weather config). Per-household weather is a follow-up task. Add a guard: if multiple households exist, log a warning that weather is shared.
- **WebSocket + expired session**: If a session expires while WebSocket is connected, the connection stays alive. Acceptable — next HTTP request triggers re-auth.
- **Active user cookie on login/switch**: Clear the `gamwich_active_user` cookie when a user logs in or switches households. A stale cookie could reference a family member from a different household.

---

## Files Summary

### New files (18)
| File | Purpose |
|------|---------|
| `internal/database/migrations/011_add_auth.sql` | Schema changes + table rebuilds |
| `internal/model/user.go` | User model |
| `internal/model/household.go` | Household + HouseholdMember models |
| `internal/model/session.go` | Session + MagicLink models |
| `internal/store/user.go` | User CRUD |
| `internal/store/household.go` | Household CRUD + membership + seed defaults |
| `internal/store/session.go` | Session management |
| `internal/store/magic_link.go` | Auth code lifecycle |
| `internal/email/postmark.go` | Postmark API client |
| `internal/auth/context.go` | Auth context helpers |
| `internal/middleware/auth.go` | Auth + admin middleware |
| `internal/middleware/ratelimit.go` | Rate limiting for auth endpoints |
| `internal/handler/auth.go` | Login, register, verify, invite, logout, household switching |
| `web/templates/login.html` | Login page |
| `web/templates/register.html` | Registration page |
| `web/templates/check_email.html` | Email sent confirmation |
| `web/templates/households.html` | Household picker (multi-household users) |

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

1. `go test ./...` — all existing tests updated with householdID, new store/middleware/email/ratelimit tests pass
2. New tests verify cross-tenant isolation: data from household A never appears in household B queries
3. Manual flow: register -> check email -> enter 6-digit code -> session created -> dashboard loads -> invite another user -> they enter code -> both see same household data
4. HTMX test: expired session on HTMX request returns `HX-Redirect: /login` (not HTML swap)
5. WebSocket test: broadcasts only reach clients in the same household
6. UNIQUE constraint test: two households can both have a family member named "Mom" and a chore area named "Kitchen"
7. Rate limiting test: rapid auth code requests return 429 after threshold
8. Household switching test: user in multiple households can switch, active user cookie is cleared on switch
9. Cookie security test: session cookie has Secure flag when served over HTTPS

## Implementation Order
Phase 1 -> Phase 2 -> Phase 3 -> Phase 4 -> Phase 5 (strictly sequential, each builds on the last)

---

## Future: Cloud Tier Integration Points

This auth plan provides the foundation for paid tier features. The following hooks are needed but out of scope here (see `dev/business-plan.md`):

- **License/subscription model**: Cloud tier activation needs a way to verify a household's subscription status. Could be a `subscription` table or an API call to a billing service.
- **Tunnel relay**: The remote access tunnel authenticates using the session token — no additional auth layer needed. The tunnel client needs to know the household's relay endpoint.
- **Encrypted backups**: The backup agent runs within the authenticated app context. Backup encryption keys should be derived from a user-held secret, not stored server-side.
- **Push notifications**: Requires a `push_subscriptions` table (user_id, household_id, endpoint, keys) and a Web Push integration. Auth context provides the scoping.
- **Hosted tier**: For multi-tenant hosted infrastructure, the same auth system works. The hosted platform provisions a new Gamwich instance per household (or uses a shared instance with the multi-tenancy from this plan).
