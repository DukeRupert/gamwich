# Gamwich Architecture Reference

Portable reference for working on Gamwich from any machine. Covers patterns, key files, testing conventions, and project status.

## Architecture Patterns

### Templating & Layout
- Go's `template.ParseGlob` can't have multiple `{{define "content"}}` blocks. Solution: render section partials to `bytes.Buffer`, pass as `template.HTML` via `data["Content"]`.
- Single `layout.html` outputs `{{.Content}}` directly. Section templates define named blocks like `{{define "dashboard-content"}}`.
- **Template FuncMap**: Uses `template.New("").Funcs(funcMap).ParseGlob(...)` — `add` function for grid math, `formatBytes`, `seq`.
- **Template `dict` limitation**: FuncMap only has `add`. Cannot use `dict` helper. Inline card rendering inside `{{range}}` using `$.ParentField` to access parent context.

### Navigation & HTMX
- Bottom nav uses `hx-get` + `hx-push-url`. Full-page routes also exist for direct navigation/browser back.
- **View transitions**: `hx-swap="innerHTML transition:true"` on all 6 bottom nav buttons. CSS `::view-transition-old/new(root)` with 100ms fade.

### Modal Pattern
All modals follow the same pattern — HX-Trigger header + JS listener to close dialog:
- `closeEventModal` (calendar)
- `closeChoreModal` (chores)
- `closeGroceryModal` (grocery)
- `closeNoteModal` (notes)
- `closeRewardModal` (rewards)
- PIN challenge uses the same dialog modal approach.

### Authentication
- **Active user**: Cookie-based (`gamwich_active_user`), 30-day expiry. PIN members use dialog modal.
- **Magic links**: Postmark API for transactional emails. 15-minute expiry, one-time use.
- **Sessions**: 90-day cookie (`gamwich_session`), HttpOnly, SameSite=Lax, Secure on HTTPS.

### Real-Time Sync
- **WebSocket**: Hub pattern in `internal/websocket/` — mutex-based client tracking, JSON broadcast. Handlers have nil-safe `broadcast()` helper.
- **Client-side**: Vanilla JS with exponential backoff reconnect, scoped HTMX re-fetches based on `document.body.dataset.activeSection`.
- **Settings sync**: WebSocket `settings` entity triggers `window.location.reload()` on other screens.
- **Server timeouts**: `ReadHeaderTimeout` + `IdleTimeout` only (no Read/WriteTimeout) to support long-lived WebSocket connections.

### Configuration: App vs Tenant

**App-level (env vars only, no DB/UI):**
- `GAMWICH_POSTMARK_TOKEN` — Postmark server token
- `GAMWICH_FROM_EMAIL` — Sender email address
- `GAMWICH_BASE_URL` — Base URL for auth code emails
- `GAMWICH_DB_PATH`, `GAMWICH_PORT` — Core infrastructure
- `GAMWICH_LICENSE_KEY`, `GAMWICH_LICENSE_URL` — License/billing

**Tenant-level (DB-overrides-env, configurable via Settings UI):**
- Weather: latitude, longitude, units
- S3 backup: endpoint, bucket, region, access key, secret key
- VAPID: public key, private key (auto-generated + persisted on first startup)
- Theme: mode, selected, light, dark
- Kiosk: idle timeout, quiet hours, burn-in prevention
- Tunnel: enabled, token
- Backup: enabled, schedule hour, retention days, passphrase

Migration 017 removed email keys from the settings table to enforce this boundary.

### Hot-Reload
- Backup `Manager.UpdateS3Config()` uses `sync.RWMutex` for concurrent-safe config swap. `runBackup` copies fields under RLock, then releases before use.
- Email client config is immutable after startup (set once from env vars, no `UpdateConfig`).

### Data Patterns
- **Chore status**: Pure Go logic in `internal/chore/status.go` — reuses `recurrence.Expand()` for due-date computation. Statuses: pending, completed, overdue, not_due.
- **Chore areas**: `chore_areas` table with FK `area_id` on `chores` (ON DELETE SET NULL). Seed data: Kitchen, Bathroom, Bedroom, Yard, General.
- **Grocery categories**: TEXT column on items (not FK). `grocery_categories` table provides pick-list + sort order. Auto-categorization via `internal/grocery/categorize.go`.
- **Note priority**: TEXT column (urgent|normal|fun), color-coded left border (red/blue/green).
- **Note expiry**: Lazy cleanup — `DeleteExpired()` called at start of `List()`. No background goroutine.
- **Reward points**: Snapshotted into `chore_completions.points_earned` at completion time. Balance = earned - spent.
- **Reward sub-page**: Lives under Chores (`/chores/rewards`), admin in Settings. Not a separate nav tab.
- **Leaderboard**: Dashboard widget, togglable via `rewards_leaderboard_enabled` setting.

### Recurrence
- **Exception model**: Parent recurring event + exception rows (modified/cancelled occurrences). Exception has `recurrence_parent_id`, `original_start_time`, `cancelled` flag.
- **Virtual expansion**: `expandEventsForRange()` in template.go merges non-recurring + expanded recurring events, applies exceptions, returns sorted list.
- **Edit/delete modes**: `single` (create exception), `all` (update parent + delete exceptions or delete parent with CASCADE). Choice dialog templates shown for recurring events.
- **ListByDateRange exclusion**: Filters out recurring parents and exception events (`recurrence_rule = '' AND recurrence_parent_id IS NULL`).

### Kiosk & Display
- **Kiosk idle mode**: Alpine.js in layout.html — activity listeners reset idle timer, overlay with large clock + next event via HTMX polling. `x-show="isIdle"` with fade transitions.
- **Night mode**: Minutes-since-midnight comparison with overnight wrap support. CSS `filter: brightness(0.15)` on body, idle overlay uses conditional opacity classes.
- **Burn-in prevention**: Sinusoidal 2px pixel shift via CSS `transform: translate()` every 150s during idle mode.
- **Theme system**: `themeKeys` in settings store. FOUC prevention via synchronous `<script>` before `<head>`. Alpine.js `initTheme()`/`applyTheme()` handles manual/system/quiet-hours modes. `matchMedia` listener for system mode. Custom `gamwich` theme defined inline with OKLCH vars.

### Frontend
- **Custom pickers**: Alpine.js manages date/time state; hidden inputs pass values to server on form submit.
- **Sensitive field masking**: Settings UI uses Alpine.js `showToken`/`showKeys` toggles with `:type="show ? 'text' : 'password'"` pattern.
- **Weather**: In-memory cache with `sync.RWMutex`, 30min TTL, lazy fetch. Uses `baseURL` field for test overriding.
- **VAPID auto-persist**: On first startup, if no VAPID keys in env or DB, auto-generate and `settingsStore.Set()` to persist.

## Key Files

### Handlers
| File | Purpose |
|------|---------|
| `internal/handler/template.go` | All template/HTMX handlers, `buildDashboardData()`, calendar/chore view builders |
| `internal/handler/calendar_event.go` | Calendar REST API (JSON) |
| `internal/handler/chore.go` | Chore REST API (JSON) |
| `internal/handler/grocery.go` | Grocery REST API (JSON) |
| `internal/handler/note.go` | Notes REST API (JSON) |
| `internal/handler/reward.go` | Rewards REST API (JSON, redeem, points, leaderboard) |
| `internal/handler/settings.go` | Settings REST API (JSON, kiosk validation) |

### Stores
| File | Purpose |
|------|---------|
| `internal/store/calendar_event.go` | EventStore with CRUD, recurrence, exceptions, ListByDateRange |
| `internal/store/chore.go` | ChoreStore with area/chore/completion CRUD |
| `internal/store/grocery.go` | GroceryStore with category/list/item CRUD, toggle, clear |
| `internal/store/note.go` | NoteStore with CRUD, pinned, expiry, lazy cleanup |
| `internal/store/reward.go` | RewardStore with CRUD, redemptions, point balances |
| `internal/store/settings.go` | SettingsStore with Get/Set/GetAll/GetKioskSettings |

### Pure Logic
| File | Purpose |
|------|---------|
| `internal/chore/status.go` | ComputeStatus(), IsDueOnDate(), status constants |
| `internal/recurrence/rrule.go` | RRULE parsing, serialization, human-readable description |
| `internal/recurrence/expand.go` | Occurrence expansion within date ranges |
| `internal/grocery/categorize.go` | Categorize() with exact + substring matching |
| `internal/weather/weather.go` | Open-Meteo integration with caching |

### WebSocket
| File | Purpose |
|------|---------|
| `internal/websocket/hub.go` | Hub: client tracking, broadcast, Message type |
| `internal/websocket/client.go` | Client: read/write pumps, ping |
| `internal/websocket/handler.go` | HTTP to WebSocket upgrade handler |

### Templates
| File | Purpose |
|------|---------|
| `web/templates/layout.html` | Dashboard shell (status bar, content area, bottom nav, modals) |
| `web/templates/calendar.html` | Day view, week view, event detail, event form, edit form |
| `web/templates/chores.html` | All chore named templates (list views, card, forms, manage, area list, summary widget) |
| `web/templates/grocery_placeholder.html` | All grocery named templates (content, item list, category groups, items, edit form, summary widget) |
| `web/templates/notes.html` | All note named templates (content, list, card, forms, summary widget) |
| `web/templates/rewards.html` | All reward named templates (content, list, leaderboard widget, manage, forms) |
| `web/templates/settings.html` | Settings page + all settings form templates |
| `web/templates/family_members.html` | Family member named templates (member-list, pin-*, toast-*) |
| `web/templates/user_select.html` | User select bar, weather widget, pin-challenge |
| `web/templates/idle.html` | idle-next-event template for screensaver |

### Other
| File | Purpose |
|------|---------|
| `web/static/manifest.json` | PWA manifest (name, icons, standalone display) |
| `web/static/sw.js` | Service worker (network-first, CDN pre-cache) |
| `docs/kiosk-setup.md` | Raspberry Pi kiosk deployment guide |

## Testing Notes

- No CGO needed — uses `modernc.org/sqlite` pure Go driver
- Store tests need explicit `PRAGMA foreign_keys = ON` for `:memory:` DBs (modernc/sqlite DSN param not honored)
- Weather tests use `baseURL = "http://127.0.0.1:1"` to force fetch failures for cache testing
- **GetAllPointBalances fix**: Must collect member IDs first, close rows, then query earned/spent — nested queries fail while rows iterator is open in modernc/sqlite

### Test Coverage by Package

| Package | Coverage |
|---------|----------|
| Calendar store | CRUD, date range overlap, all-day ordering, spanning events, FK cascade SET NULL, recurrence CRUD, exceptions, cascade delete |
| Chore store | Area seed data, area CRUD, chore CRUD, list by assignee/area, FK cascade (delete chore->completions, delete member->SET NULL, delete area->SET NULL), completion CRUD, date range queries, sort order |
| Chore status | One-off pending/completed, daily pending/completed/overdue, weekly not-due/due/overdue, biweekly, monthly, invalid rule fallback, IsDueOnDate |
| Recurrence | RRULE parsing/serialization/round-trip, daily/weekly/biweekly/monthly/yearly expansion, BYDAY, COUNT, UNTIL, range filtering, duration preservation |
| Grocery categorize | Exact match, substring match, case insensitivity, empty string, whitespace trimming, unknown items |
| Grocery store | Seed data (11 categories, 1 default list), item CRUD, not found, list ordering, checked after unchecked, toggle check/uncheck, clear checked, count unchecked, FK CASCADE (delete list->items), FK SET NULL (delete member->added_by/checked_by), nil added_by |
| WebSocket hub | Register/unregister, double unregister, broadcast to all clients, empty hub broadcast, full buffer message drop, NewMessage type construction, concurrent access |
| Settings store | Seed data (5 kiosk + 3 weather + 4 theme + 5 s3 + 2 vapid = 19 settings), get/set/update, get-not-found, GetKioskSettings, GetThemeSettings, GetAll |
| Note store | CRUD, not-found, list ordering, expired exclusion, ListPinned, TogglePinned, DeleteExpired, FK SET NULL (delete author), nil author, with-expiry |
| Reward store | CRUD, not-found, list ordering, ListActive, Redeem, ListRedemptionsByMember, PointBalance (earned-spent), no-activity, GetAllPointBalances, FK CASCADE (delete reward->redemptions), FK SET NULL (delete member->redemption), leaderboard setting seed |

## Completed Milestones

| Version | Milestone | Summary |
|---------|-----------|---------|
| 1.1 | Project scaffold | Family members (CRUD, PIN system) |
| 1.2 | Dashboard shell | Navigation, weather widget, user selection |
| 1.3 | Calendar basics | CRUD, day/week views, quick-add form |
| 1.4 | Recurring events | RRULE, exceptions, edit/delete modes, choice dialogs |
| 1.5 | Chore Lists | CRUD, areas, completions, status logic, views, dashboard widget |
| 1.6 | Grocery Lists | CRUD, auto-categorization, check-off, clear, category groups, dashboard widget |
| 1.7 | Real-Time Sync | WebSocket hub, broadcasts in all handlers, client-side scoped refresh |
| 1.8 | Mobile Companion | PWA manifest/service worker, responsive CSS (375px-1920px), network status |
| 1.9 | Kiosk Polish | Idle/screensaver, night mode, burn-in prevention, settings table, kiosk docs |
| 2.1 | Notes | CRUD, pinned notes, priority colors, auto-expire, dashboard widget, 6th nav tab |
| 2.2 | Rewards | Chore points, rewards catalog, redemption, leaderboard, settings management |
| 3.5 | Theming | DaisyUI themes (5 incl. custom), auto dark mode, view transitions, animations |
| 4.2 | Remote Access | Cloudflare Tunnel integration |
| 4.3 | Backups | Encrypted offsite S3 backups |
| 4.4 | Push Notifications | VAPID web push with scheduling |
| 4.5 | Settings UI | UI-configurable settings for S3, VAPID; email moved to env-only (v0.18.0) |
