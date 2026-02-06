# Gamwich — Implementation Plan

> Detailed implementation steps for building the Gamwich family organizer app. Each milestone is a deployable increment. Tasks within each milestone are ordered for incremental development and testable commits.

---

## Phase 1: MVP — The Fridge Door Replacement

The goal of Phase 1 is a working app that a family can actually use on a kitchen touchscreen to replace paper lists and fridge-door clutter. Every milestone in this phase ends with something usable.

---

### Milestone 1.1: Project Scaffold & Family Members ✅

**Goal:** Go project structure, database, and the ability to manage family members. The foundation everything else builds on.

**Status:** Complete

#### Tasks

1. **~~Initialize Go module and project structure~~** ✅
   - `go mod init gamwich`
   - Establish directory layout: `cmd/gamwich/`, `internal/`, `web/templates/`, `web/static/`, `db/migrations/`
   - Create `main.go` with basic HTTP server startup
   - _Test: `go run` starts and serves a "Hello Gamwich" page_

2. **~~Set up SQLite database layer~~** ✅
   - Choose driver (`modernc.org/sqlite` for pure Go, no CGO)
   - Create migration runner (embed SQL files with `embed` package)
   - Write initial migration: `family_members` table (`id`, `name`, `color`, `avatar_emoji`, `pin`, `sort_order`, `created_at`, `updated_at`)
   - _Test: App starts, runs migrations, DB file created_

3. **~~Create family member CRUD API~~** ✅
   - `POST /api/family-members` — create
   - `GET /api/family-members` — list all
   - `PUT /api/family-members/{id}` — update
   - `DELETE /api/family-members/{id}` — delete
   - Validate: name required, color format (hex), unique name
   - _Test: curl/httpie against all endpoints_

4. **~~Build family member management page~~** ✅
   - Go template layout: base template with nav shell, content block
   - Tailwind CSS + DaisyUI setup (CDN for now, build step later)
   - DaisyUI provides the component foundation: large buttons, cards, badges, modals, form inputs — all touch-friendly out of the box
   - Use DaisyUI themes for consistent styling (start with `cupcake` or `garden` for a warm, friendly default)
   - Family member list with colored avatars (DaisyUI `avatar` + `badge` components)
   - htmx-powered add/edit/delete forms (inline editing, no page reloads)
   - Color picker for member color assignment
   - Drag-to-reorder with Alpine.js (sort_order) — _API ready, UI drag not yet wired_
   - Global toast notification system (htmx OOB swaps, auto-dismiss)
   - _Test: Add, edit, reorder, and delete family members from the browser_

5. **~~Implement PIN selection for family members~~** ✅
   - Optional 4-digit PIN per member
   - PIN entry component (large touchscreen-friendly number pad)
   - Store PIN hashed (bcrypt) in DB
   - Changing or removing an existing PIN requires verifying the current PIN first
   - _Test: Set a PIN, verify PIN entry works on the family member page_

---

### Milestone 1.2: Dashboard Shell & Navigation ✅

**Goal:** The main touchscreen interface — a dashboard layout that will host all the widgets, plus navigation between sections.

**Status:** Complete

#### Tasks

1. **Create the dashboard layout template**
   - Full-screen layout optimized for landscape 1920x1080 (typical kitchen touchscreen)
   - Top bar: date, time (live-updating with Alpine.js), weather placeholder
   - Main content area: grid layout using DaisyUI `card` components for each dashboard widget
   - Bottom nav: DaisyUI `btm-nav` component — Calendar, Chores, Grocery, Settings — naturally large touch targets
   - Use DaisyUI `btn-lg` or `btn-xl` size as the minimum for all interactive elements
   - _Test: Dashboard loads, clock ticks, nav buttons are present and tappable_

2. **Implement section routing with htmx**
   - Bottom nav triggers htmx requests to swap main content area
   - URL updates with `hx-push-url` for browser back/forward support
   - Active nav state styling
   - _Test: Tap between sections, URL updates, back button works_

3. **Add weather widget**
   - Configurable location (lat/lon in settings)
   - Fetch weather from a free API (Open-Meteo — no API key required)
   - Cache weather data server-side (refresh every 30 minutes)
   - Display: current temp, condition icon, high/low for today
   - Fallback: graceful "no weather data" state if network is down
   - _Test: Weather displays on dashboard, updates periodically_

4. **Build the "Who am I?" quick-select**
   - Family member avatar bar along one edge of the dashboard (DaisyUI `avatar-group` with `avatar` components)
   - Tap to set "active user" — determines attribution for adds/completions
   - PIN challenge using DaisyUI `modal` with large `kbd`-styled number buttons
   - Store active user in browser session/cookie (per-device, not server session)
   - Active user highlighted with DaisyUI `ring` or `outline` on their avatar
   - _Test: Tap avatar, enter PIN if needed, active user indicator updates_

---

### Milestone 1.3: Calendar — Basic CRUD

**Goal:** View and manage calendar events. Day and week views with create/edit/delete.

#### Tasks

1. **Create calendar database tables**
   - Migration: `calendar_events` table (`id`, `title`, `description`, `start_time`, `end_time`, `all_day`, `family_member_id`, `location`, `created_at`, `updated_at`)
   - Index on `start_time` for range queries
   - _Test: Migration runs cleanly_

2. **Build calendar event API**
   - `POST /api/events` — create event
   - `GET /api/events?start={date}&end={date}` — list events in date range
   - `GET /api/events/{id}` — single event
   - `PUT /api/events/{id}` — update
   - `DELETE /api/events/{id}` — delete
   - Validate: title required, start before end, family_member_id exists
   - _Test: CRUD operations via curl_

3. **Build day view**
   - Vertical timeline from 6am to 10pm (configurable)
   - Events rendered as colored blocks (color = family member color)
   - All-day events as banners at the top
   - Tap empty time slot → quick-add form (htmx modal or inline)
   - Tap event → view/edit/delete panel
   - _Test: Create events, see them on the day view, edit and delete_

4. **Build week view**
   - 7-column grid, current day highlighted
   - Events shown as compact colored bars
   - Tap a day → navigate to day view
   - Swipe left/right to navigate weeks (Alpine.js touch handler)
   - _Test: Week view renders events across multiple days, swipe navigation works_

5. **Quick-add event form**
   - DaisyUI `modal` with large, touch-friendly form inputs (`input-lg`, `select-lg`)
   - Fields: title, date/time pickers, family member selector (DaisyUI `select` or custom avatar buttons), optional notes
   - Date/time picker designed for touch (scroll wheels or large increment buttons, NOT native browser pickers)
   - htmx submit → event appears on calendar without page reload
   - _Test: Add event from quick-add, verify it appears immediately_

---

### Milestone 1.4: Calendar — Recurring Events

**Goal:** Support for repeating events (weekly soccer practice, monthly book club, birthdays).

#### Tasks

1. **Add recurrence to the data model**
   - Add `recurrence_rule` column to `calendar_events` (RFC 5545 RRULE subset: FREQ, INTERVAL, BYDAY, BYMONTHDAY, COUNT, UNTIL)
   - Add `recurrence_parent_id` for exception tracking
   - _Test: Migration runs_

2. **Implement recurrence expansion logic**
   - Go package to expand an RRULE into concrete occurrences within a date range
   - Support: daily, weekly (with BYDAY), biweekly, monthly (by day-of-month), yearly
   - Return virtual event instances (not stored individually)
   - _Test: Unit tests — "every Tuesday" returns correct dates for a given range_

3. **Update event API to handle recurrence**
   - Event list endpoint expands recurring events within the requested range
   - Edit options: "this occurrence", "this and future", "all occurrences"
   - Delete options: same three choices
   - Store exceptions as separate events linked by `recurrence_parent_id`
   - _Test: Create recurring event, verify occurrences appear on calendar, edit single occurrence_

4. **Update UI for recurring events**
   - Recurrence selector on the event form (None, Daily, Weekly, Biweekly, Monthly, Yearly, Custom)
   - Visual indicator (repeat icon) on recurring events in calendar views
   - Edit/delete dialog with "this one / future / all" options
   - _Test: Full round-trip — create recurring, view occurrences, modify one, delete the series_

---

### Milestone 1.5: Chore Lists

**Goal:** Assign, track, and complete household chores.

#### Tasks

1. **Create chore database tables**
   - Migration: `chores` table (`id`, `title`, `description`, `area`, `points`, `recurrence_rule`, `assigned_to`, `sort_order`, `created_at`, `updated_at`)
   - Migration: `chore_completions` table (`id`, `chore_id`, `completed_by`, `completed_at`)
   - Migration: `chore_areas` table (`id`, `name`, `sort_order`) with seed data (Kitchen, Bathroom, Bedroom, Yard, General)
   - _Test: Migrations run_

2. **Build chore CRUD API**
   - `POST /api/chores` — create chore
   - `GET /api/chores` — list with filters (assignee, area, status)
   - `PUT /api/chores/{id}` — update
   - `DELETE /api/chores/{id}` — delete
   - `POST /api/chores/{id}/complete` — mark complete (creates completion record)
   - `DELETE /api/chores/{id}/completions/{completion_id}` — undo completion
   - _Test: CRUD + completion via curl_

3. **Implement chore status logic**
   - Determine if a chore is "due today" based on its recurrence rule and last completion date
   - States: pending, completed today, overdue (due date passed without completion)
   - Query helper: "what's due today for family member X?"
   - _Test: Unit tests for due-date calculation with various recurrence patterns_

4. **Build chore list page**
   - Default view: today's chores grouped by family member
   - Each chore rendered as a DaisyUI `card` with: title, area `badge`, assignee color, `checkbox` (use `checkbox-lg` for easy touch)
   - Tap checkbox → DaisyUI `toast` or inline animation + completion recorded (htmx swap)
   - Overdue chores use DaisyUI `alert-warning` or `alert-error` styling
   - Filter tabs: DaisyUI `tabs` component — By Person, By Area, All
   - _Test: View chores, complete them, see status update in real-time_

5. **Build chore management page**
   - Add/edit chore form: title, description, area, assignee, recurrence, points (optional)
   - Area management: add/rename/reorder areas
   - _Test: Create chores with various recurrence rules, verify they appear on the right days_

6. **Add chore summary widget to dashboard**
   - Compact view: each family member's name/avatar with "3 of 5 done" progress
   - Tap to navigate to full chore list filtered by that person
   - _Test: Dashboard shows accurate chore counts, tapping navigates correctly_

---

### Milestone 1.6: Grocery Lists

**Goal:** Shared grocery list that can be managed from the kitchen screen and checked off at the store.

#### Tasks

1. **Create grocery database tables**
   - Migration: `grocery_lists` table (`id`, `name`, `sort_order`, `created_at`)
   - Migration: `grocery_items` table (`id`, `list_id`, `name`, `quantity`, `unit`, `notes`, `category`, `checked`, `checked_by`, `checked_at`, `added_by`, `sort_order`, `created_at`)
   - Migration: `grocery_categories` table (`id`, `name`, `sort_order`) with seed data (Produce, Dairy, Meat & Seafood, Bakery, Pantry, Frozen, Beverages, Snacks, Household, Personal Care, Other)
   - Seed a default "Grocery" list
   - _Test: Migrations run, default list and categories exist_

2. **Build grocery item API**
   - `POST /api/grocery-lists/{list_id}/items` — add item
   - `GET /api/grocery-lists/{list_id}/items` — list items (grouped by category)
   - `PUT /api/grocery-lists/{list_id}/items/{id}` — update
   - `DELETE /api/grocery-lists/{list_id}/items/{id}` — delete
   - `POST /api/grocery-lists/{list_id}/items/{id}/check` — toggle checked
   - `POST /api/grocery-lists/{list_id}/clear-checked` — remove all checked items
   - _Test: Full CRUD + check/uncheck via curl_

3. **Implement auto-categorization**
   - Maintain a Go map of common grocery items → category (e.g., "milk" → Dairy, "chicken" → Meat)
   - On item add, if no category specified, look up the item name
   - Fallback to "Other" if no match
   - Allow manual override
   - _Test: Add "bananas" → auto-categorized as Produce_

4. **Build grocery list page**
   - Items grouped by category with DaisyUI `collapse` sections (tap to expand/collapse)
   - Each item: name, quantity/notes, `checkbox-lg`, `btn-ghost` delete
   - Quick-add bar at top: DaisyUI `input-lg` with `join` button group for add action — prominent and fast
   - Tap checkbox → item moves to "checked" section at bottom (strikethrough, dimmed via `opacity-40`)
   - "Clear checked" button (`btn-lg btn-outline`) to remove purchased items
   - _Test: Add items, check them off, clear checked items_

5. **Add grocery count widget to dashboard**
   - Badge showing unchecked item count on the default list
   - Tap to navigate to full grocery list
   - _Test: Badge updates as items are added/checked_

---

### Milestone 1.7: Real-Time Sync with WebSockets

**Goal:** When someone adds a grocery item from their phone, the kitchen screen updates instantly.

#### Tasks

1. **Set up WebSocket server**
   - WebSocket endpoint: `GET /ws`
   - Hub pattern: track connected clients, broadcast messages
   - Message format: JSON `{ "type": "event_created", "entity": "grocery_item", "id": 42, "list_id": 1 }`
   - Reconnect logic on client side (Alpine.js or vanilla JS)
   - _Test: Two browser tabs connected, verify both receive messages_

2. **Integrate WebSocket broadcasts into API handlers**
   - After every successful create/update/delete, broadcast a change notification
   - Include enough data for the client to know what changed (entity type, ID, action)
   - _Test: Add a grocery item in one tab, see it appear in another_

3. **Implement client-side htmx refresh on WebSocket message**
   - On receiving a relevant WebSocket message, trigger an htmx request to re-fetch the affected section
   - Scope refreshes narrowly: grocery change only refreshes grocery list, not the whole page
   - Dashboard widgets refresh independently based on message type
   - _Test: Full round-trip — add event on phone, kitchen screen calendar updates without manual refresh_

---

### Milestone 1.8: Responsive Mobile Companion

**Goal:** The same app works well on a phone browser for on-the-go use (checking grocery list at the store, adding events from work).

#### Tasks

1. **Responsive layout audit and fixes**
   - Ensure all pages work at 375px width (iPhone SE) through 1920px (kitchen screen)
   - Dashboard: stack widgets vertically on mobile
   - Calendar: day view as default on mobile, week view scrollable
   - Grocery list: optimized for one-handed phone use at the store
   - Navigation: bottom tab bar on mobile, side or bottom on desktop/touchscreen
   - _Test: All pages usable on phone-sized viewport_

2. **Touch interaction audit**
   - Verify all interactive elements use DaisyUI `-lg` or `-xl` size variants at minimum
   - Swipe gestures work on mobile browsers
   - No hover-dependent interactions (DaisyUI's focus/active states handle this well)
   - _Test: Navigate entire app on a real phone_

3. **Add to Home Screen / PWA basics**
   - `manifest.json` with app name, icon, theme color
   - Service worker for offline shell caching (app loads even if server is briefly unreachable)
   - Fullscreen display mode when added to home screen
   - _Test: Add to home screen on Android/iOS, app opens fullscreen_

4. **Network-aware behavior**
   - Show connection status indicator when WebSocket disconnects
   - Queue actions when offline, replay when reconnected (stretch — at minimum, show clear "you're offline" state)
   - _Test: Disconnect WiFi, app shows offline state, reconnect restores_

---

### Milestone 1.9: Touchscreen Kiosk Polish

**Goal:** Optimize the experience for a dedicated kitchen touchscreen running in kiosk mode.

#### Tasks

1. **Idle / screensaver mode**
   - After configurable inactivity timeout (default 5 minutes), transition to idle mode
   - Idle mode: dimmed screen showing clock, date, and next upcoming event
   - Any touch returns to full dashboard
   - CSS `prefers-reduced-motion` respected
   - _Test: Leave app idle, screen dims, tap to wake_

2. **Night mode scheduling**
   - Configurable quiet hours (e.g., 10pm–6am)
   - During quiet hours: very dim display, minimal content
   - _Test: Set quiet hours, verify dimming activates on schedule_

3. **Kiosk deployment documentation**
   - Raspberry Pi setup guide: OS, browser in kiosk mode, auto-start on boot
   - Chromium flags for kiosk: `--kiosk --noerrdialogs --disable-infobars`
   - Auto-restart on crash
   - Touchscreen calibration notes
   - _Test: Follow guide on a fresh Raspberry Pi, app runs in kiosk mode on boot_

4. **Burn-in prevention**
   - Subtle periodic pixel shift (1-2px every few minutes) during idle
   - Alternating dark/light elements in idle mode
   - _Test: Verify pixel shift is imperceptible but measurable_

---

## Phase 2: Quality of Life

Phase 2 adds features that make the app stickier and more connected to the family's existing digital life.

---

### Milestone 2.1: CalDAV Integration

**Goal:** Bidirectional sync with Google Calendar, iCloud, or any CalDAV server.

#### Tasks

1. **Research and select Go CalDAV library**
   - Evaluate `emersion/go-webdav` or similar
   - Determine if acting as CalDAV client (syncing FROM Google/iCloud) or server (others sync TO Gamwich), or both
   - Document decision and limitations
   - _Test: Spike — connect to a test CalDAV server, list calendars_

2. **Build CalDAV client sync**
   - Settings page: add CalDAV account (URL, username, password/token)
   - Periodic sync job (configurable interval, default 15 minutes)
   - Map external events to Gamwich events, track sync state (etag/ctag)
   - Handle conflicts: external wins by default, with option to flag for review
   - _Test: Connect to Google Calendar via CalDAV, events appear in Gamwich_

3. **Build outbound sync (Gamwich → CalDAV)**
   - Events created/modified in Gamwich push to the linked CalDAV calendar
   - Respect sync direction settings (one-way in, one-way out, bidirectional)
   - _Test: Create event in Gamwich, verify it appears in Google Calendar_

4. **Calendar source indicators in UI**
   - Visual badge on events showing origin (Gamwich, Google, iCloud, etc.)
   - External events may be read-only depending on sync settings
   - _Test: Mixed calendar view shows events from multiple sources with clear attribution_

---

### Milestone 2.2: Meal Planning

**Goal:** Plan meals for the week, display tonight's dinner on the dashboard, and push ingredients to the grocery list.

#### Tasks

1. **Create meal planning database tables**
   - Migration: `meals` table (`id`, `date`, `meal_type`, `title`, `recipe_url`, `notes`, `created_at`, `updated_at`)
   - Migration: `meal_ingredients` table (`id`, `meal_id`, `name`, `quantity`, `unit`, `category`)
   - `meal_type` enum: breakfast, lunch, dinner, snack
   - _Test: Migrations run_

2. **Build meal planning API**
   - `POST /api/meals` — create/assign a meal to a date
   - `GET /api/meals?start={date}&end={date}` — list meals in range
   - `PUT /api/meals/{id}` — update
   - `DELETE /api/meals/{id}` — remove from plan
   - `POST /api/meals/{id}/to-grocery` — push ingredients to grocery list
   - _Test: CRUD + grocery push via curl_

3. **Build weekly meal planning page**
   - 7-column grid (like week calendar view), rows for meal types
   - Tap empty slot → add meal (title, optional recipe URL, optional notes)
   - Tap filled slot → edit, delete, or "add ingredients to grocery list"
   - _Test: Plan a week of dinners, push ingredients to grocery list_

4. **Build family favorites**
   - `saved_meals` table: title, recipe_url, notes, ingredients, use_count
   - "Save as favorite" option when adding a meal
   - Quick-add from favorites when planning
   - _Test: Save a meal, reuse it on another day_

5. **Add "What's for dinner?" widget to dashboard**
   - Prominent display of tonight's planned meal
   - Shows meal title and recipe link if available
   - Fallback: "No dinner planned — tap to add"
   - _Test: Plan tonight's dinner, verify it shows on dashboard_

---

### Milestone 2.3: Rewards & Points System

**Goal:** Optional gamification to motivate chore completion, especially for kids.

#### Tasks

1. **Add points infrastructure**
   - `points` column already exists on `chores` table
   - Migration: `rewards` table (`id`, `title`, `description`, `point_cost`, `active`, `created_at`)
   - Migration: `reward_redemptions` table (`id`, `reward_id`, `redeemed_by`, `redeemed_at`)
   - _Test: Migrations run_

2. **Build points tracking**
   - On chore completion, credit points to the completing family member
   - `GET /api/family-members/{id}/points` — current balance (sum of earned minus redeemed)
   - Points history view
   - _Test: Complete chores, verify point balance increases_

3. **Build reward management**
   - Settings page: add/edit/delete rewards with point costs
   - Rewards page: family members see available rewards and their point balance
   - Redeem button (with confirmation)
   - _Test: Create reward, earn enough points via chores, redeem reward_

4. **Add points/rewards UI elements**
   - Points balance on family member avatars (dashboard)
   - "You earned X points!" feedback on chore completion
   - Optional leaderboard widget (can be disabled in settings)
   - _Test: Full flow visible in UI — complete chore, see points, redeem reward_

---

### Milestone 2.4: Notes & Message Board

**Goal:** Replace sticky notes on the fridge with pinned digital messages.

#### Tasks

1. **Create notes database table**
   - Migration: `notes` table (`id`, `title`, `body`, `author_id`, `pinned`, `priority`, `expires_at`, `created_at`, `updated_at`)
   - Priority enum: urgent (red), normal (blue), fun (green)
   - _Test: Migration runs_

2. **Build notes API**
   - `POST /api/notes` — create
   - `GET /api/notes` — list (pinned first, then by created_at desc)
   - `PUT /api/notes/{id}` — update
   - `DELETE /api/notes/{id}` — delete
   - Auto-delete expired notes via periodic cleanup job
   - _Test: CRUD via curl, expired notes get cleaned up_

3. **Build notes page**
   - Card-based layout, color-coded by priority
   - Quick-add: title + body + optional expiry
   - Pin/unpin toggle
   - _Test: Create notes, pin one, verify pinned notes stay at top_

4. **Add pinned notes widget to dashboard**
   - Show pinned notes as small cards/banners on the dashboard
   - Urgent notes get prominent placement
   - _Test: Pin a note, verify it appears on dashboard_

---

### Milestone 2.5: Smart Grocery Features

**Goal:** Make the grocery list smarter with frequent items and better categorization.

#### Tasks

1. **Build frequent items tracking**
   - Track item add frequency over time
   - `GET /api/grocery-lists/{list_id}/frequent` — return top 20 most-added items
   - _Test: Add items repeatedly, frequent items list reflects usage_

2. **Build frequent items quick-add panel**
   - Row of chips/buttons above the grocery list showing frequent items
   - Tap to add with previous quantity/category
   - _Test: Frequently added item appears as a quick-add chip, tap adds it to the list_

3. **Improve auto-categorization**
   - Expand the static item → category map
   - Learn from manual category overrides (store user corrections and apply them next time)
   - _Test: Manually categorize an item, add it again later, correct category applied automatically_

4. **Checked items archive / re-add**
   - After clearing checked items, store them in a "recently purchased" list
   - Easy re-add: "You bought these last time" panel
   - _Test: Clear checked items, see them in recently purchased, re-add one_

---

## Phase 3: Polish & Extended Features

Phase 3 is about refinement, automation, and making the app delightful for long-term daily use.

---

### Milestone 3.1: Chore Rotation & Automation

**Goal:** Chores automatically rotate between family members on a schedule.

#### Tasks

1. **Add rotation support to data model**
   - Add `rotation_members` (JSON array of family_member_ids) and `rotation_interval` to `chores` table
   - Add `current_rotation_index` to track position
   - _Test: Migration runs_

2. **Implement rotation logic**
   - When a rotating chore's period elapses, advance the assignee to the next member in the list
   - Handle edge cases: member removed from family, member temporarily skipped
   - _Test: Unit tests — rotation advances correctly over multiple periods_

3. **Rotation UI**
   - Chore form: toggle "rotate between" → select members and interval
   - Chore list: show "Your turn" badge, upcoming rotation preview ("Next week: Sarah")
   - _Test: Create rotating chore, verify assignment changes on schedule_

---

### Milestone 3.2: Multi-List Support

**Goal:** Separate lists for different stores and purposes.

#### Tasks

1. **Enable multiple grocery lists**
   - List management: create, rename, reorder, delete lists
   - Each list has its own items and categories
   - Default list configurable
   - _Test: Create "Costco" and "Hardware Store" lists alongside the default_

2. **Update grocery UI for multi-list**
   - Tab bar or dropdown to switch between lists
   - Dashboard widget shows count from the default list, with indicators for other active lists
   - _Test: Switch between lists, add items to different lists_

---

### Milestone 3.3: Import & Integrations

**Goal:** Reduce manual data entry by importing from external sources.

#### Tasks

1. **School calendar import**
   - Import `.ics` file upload
   - Parse iCal format, create events in Gamwich
   - Optionally assign to a family member
   - _Test: Upload a school calendar .ics file, events appear on the calendar_

2. **Grocery list sharing**
   - Shareable read-only link for a grocery list (for sending to someone at the store)
   - No auth required on the shared link
   - _Test: Generate share link, open in incognito, list is visible_

3. **Data export**
   - Export all data as JSON backup
   - Export grocery list as plain text
   - Export calendar as .ics
   - _Test: Export and verify data completeness_

---

### Milestone 3.4: Kiosk Deployment Tooling

**Goal:** Make it dead simple to set up a new Gamwich kitchen screen.

#### Tasks

1. **Single-binary distribution**
   - Embed all static assets, templates, and migrations into the Go binary
   - `gamwich serve` starts everything with sane defaults
   - `gamwich setup` runs an interactive first-time configuration (family name, timezone, weather location)
   - _Test: Download binary, run setup, app works with no other dependencies_

2. **Docker image**
   - Dockerfile: minimal base image, single binary, SQLite volume mount
   - `docker-compose.yml` with volume for DB persistence
   - _Test: `docker compose up`, app runs, data persists across restarts_

3. **Raspberry Pi kiosk installer script**
   - Bash script that: installs Chromium, configures kiosk mode, sets up systemd service for Gamwich, configures auto-start
   - Handles screen rotation and resolution settings
   - _Test: Run script on a fresh Raspberry Pi OS, device boots into Gamwich_

4. **Automatic backups**
   - Configurable backup schedule (daily default)
   - SQLite `.backup` command to a configurable path (local directory, mounted NAS, etc.)
   - Retain N backups with automatic cleanup
   - Settings page: backup now, restore from backup, download backup
   - _Test: Backup runs on schedule, restore works_

---

### Milestone 3.5: Visual Polish & Theming

**Goal:** Make the app look and feel polished for daily use. DaisyUI's theme system does the heavy lifting here.

#### Tasks

1. **Theme selection**
   - DaisyUI ships with 30+ themes — expose a curated subset in settings
   - Recommended defaults: `garden` (light, warm), `forest` (dark, earthy), `cupcake` (light, playful), `dracula` (dark, bold)
   - Add a custom `gamwich` theme with Shire-inspired greens and warm browns
   - Theme toggle on settings page using DaisyUI `theme-controller`
   - Persist selection per device
   - _Test: Switch themes, all pages adapt instantly via DaisyUI's `data-theme` attribute_

2. **Auto dark mode**
   - Option to follow system preference (`prefers-color-scheme`)
   - Option to follow quiet hours schedule (dark theme during night mode)
   - _Test: Toggle system dark mode, app follows_

3. **Animations and micro-interactions**
   - Chore completion: satisfying checkmark animation (CSS transition on DaisyUI `checkbox`)
   - List item add: smooth slide-in (htmx `swap` transitions)
   - Page transitions: quick crossfade via htmx `hx-swap` with transition classes
   - Keep it performant — no jank on Raspberry Pi
   - _Test: Animations feel good on both desktop and Pi_

4. **Family photo idle screen**
   - Configure a photo directory or upload photos
   - Idle mode cycles through family photos with clock overlay
   - _Test: Add photos, idle mode displays them as a slideshow_

---

### Milestone 3.6: Grocery Suggestions (Stretch)

**Goal:** Proactive reminders based on purchase history patterns.

#### Tasks

1. **Purchase history analysis**
   - Track when items are added and cleared from the grocery list
   - Calculate average purchase frequency per item
   - _Test: Sufficient history data produces frequency calculations_

2. **Suggestion engine**
   - "It's been 2 weeks since you bought eggs — add to list?"
   - Suggestions shown as dismissable cards on the grocery page
   - Configurable: on/off, sensitivity threshold
   - _Test: After establishing a pattern, suggestions appear at the right time_

3. **Suggestion UI**
   - Suggestion cards above the grocery list
   - "Add" or "Dismiss" actions
   - Dismissed suggestions don't reappear for that cycle
   - _Test: Dismiss a suggestion, verify it doesn't come back immediately_

---

## Appendix: Cross-Cutting Concerns

These apply throughout all phases and should be addressed continuously.

### Testing Strategy
- **Unit tests:** Go — domain logic, recurrence expansion, date calculations, auto-categorization
- **Integration tests:** API endpoints with test database
- **Manual testing:** Touchscreen interaction on actual device at each milestone
- **Browser testing:** Chrome (kiosk), Safari (iOS companion), Firefox

### Configuration
- **Environment-based config:** `GAMWICH_DB_PATH`, `GAMWICH_PORT`, `GAMWICH_TIMEZONE`, etc.
- **Settings page in UI:** weather location, quiet hours, default list, theme, sync intervals
- **Config file:** TOML or YAML for initial setup, overridable by env vars

### Security
- **Local network trust model:** no auth on local network by default
- **Optional basic auth:** for remote access over Tailscale/WireGuard
- **No sensitive data in the app** — this is grocery lists, not banking
- **HTTPS via reverse proxy** (Caddy recommended) or built-in option

### Performance Targets
- **Page load:** <200ms on Raspberry Pi 4
- **WebSocket latency:** <100ms for real-time updates
- **Database:** SQLite handles household scale trivially — optimize only if measured
- **Frontend:** Minimal JS, htmx keeps payload small, Alpine.js for interactivity without framework weight, DaisyUI for consistent components without custom CSS overhead