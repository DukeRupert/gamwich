# Gamwich — Feature Set

> The open-source family dashboard. Free on your own hardware, with optional cloud services for remote access and backups.

---

## Product Vision

Gamwich is a household command center — a digital refrigerator door that lives on a wall-mounted touchscreen, tablet, or countertop display. It replaces paper lists, scattered apps, and forgotten sticky notes with a single glanceable screen the whole family shares.

### Distribution Model

**Free tier (self-hosted):** The core app runs as a single Go binary on any hardware — a Raspberry Pi, an old laptop, a NAS, or a spare tablet. No account required, no internet required, no subscription. Your family's data stays on your device in a single SQLite file.

**Cloud tier (paid subscription):** For families who want remote access from work or the store, encrypted offsite backups, and push notifications. The app still runs locally — the cloud tier adds connectivity and peace of mind, not core features.

**Hosted tier (paid subscription):** For families who don't want to manage hardware. We run the instance, they use the app. Same experience, zero setup.

### Core Differentiators

- **No vendor lock-in.** The app works forever without an internet connection or a subscription. If Gamwich the company disappears, the software keeps running.
- **Bring your own hardware.** A $50 Fire tablet or a spare iPad becomes a family dashboard. No proprietary $300 device required.
- **Privacy by default.** Family routines, grocery habits, and kids' chore data never leave your home network unless you opt into cloud services.
- **Open source.** The core app is open source. Community contributions, transparency, and trust.

---

## Core Concept

Gamwich is designed to feel like a glanceable dashboard — not a phone app scaled up. The primary interaction model is quick taps and swipes by anyone in the family walking past, with a companion mobile view for on-the-go access.

---

## 1. Dashboard (Home Screen)

The always-on default view. Designed for a landscape touchscreen (10"–27").

- **Today's date, time, and weather** at the top
- **Today's calendar events** as a timeline strip showing what's happening and what's next
- **Active chore summary** — who owes what today, with completion status
- **Grocery list count** — "12 items" badge, tap to expand
- **Quick-add button** — floating action to add an event, chore, or grocery item from any screen
- **Family member avatars** along the edge — tap to filter views by person
- **Idle mode** — after inactivity, dim to a clock/photo display to reduce burn-in and kitchen glow at night

---

## 2. Family Calendar

### Views
- **Day view** — default on dashboard, vertical timeline
- **Week view** — 7-column grid, touch to drill into a day
- **Month view** — traditional grid with event dots/counts

### Features
- **Color-coded by family member** — each person gets a color, events show whose they are
- **Recurring events** — daily, weekly, biweekly, monthly, yearly with end-date or occurrence count
- **All-day events** — birthdays, holidays, trips displayed as banners
- **Quick-add from touchscreen** — tap a time slot to create an event with title, person, and optional notes
- **CalDAV sync** — bidirectional sync with external calendars (Google Calendar, iCloud, etc.) so events added on phones appear on the kitchen screen and vice versa
- **Conflict detection** — visual indicator when two family members have overlapping events
- **Countdown/upcoming** — "3 days until Dad's birthday" style callouts on dashboard

---

## 3. Chore Lists

### Structure
- **Chores assigned to family members** — each chore has an assignee (or rotates)
- **Recurring schedules** — daily, specific days of week, weekly, biweekly, monthly
- **One-off tasks** — "Fix the leaky faucet" style items with optional due date

### Interaction
- **Tap to complete** — large, satisfying checkmark animation (designed for kids too)
- **Completion streaks** — visual indicator of consecutive completions per person
- **Overdue highlighting** — missed chores float to the top with visual emphasis
- **Rotation support** — "Take out trash" rotates through family members week by week
- **Drag to reassign** — quick reassignment when someone is sick or away

### Views
- **By person** — "What does each person need to do today?"
- **By area** — Kitchen, Bathroom, Yard, etc.
- **By status** — Outstanding, completed today, overdue

### Optional: Rewards
- **Point system** — chores earn points, configurable per chore
- **Reward tiers** — family-defined rewards at point thresholds (screen time, treat, outing)
- **Leaderboard** — fun, non-punitive display of who's been crushing it

---

## 4. Grocery / Shopping Lists

### Core
- **Shared list** — anyone can add items from the touchscreen or their phone
- **Categorized by store section** — Produce, Dairy, Meat, Pantry, Frozen, Household, etc.
- **Auto-categorize** — "Milk" automatically files under Dairy
- **Quantity and notes** — "Bananas x6 (green ones)"
- **Check-off at the store** — items can be checked off from the mobile companion view while shopping
- **Checked items archive** — recently bought items are easy to re-add with a tap

### Smart Features
- **Frequent items** — quick-add panel of things you buy regularly
- **Suggestions** — "You usually buy eggs every 2 weeks, need more?" (stretch goal)
- **Multiple lists** — Grocery, Hardware Store, Costco, etc.

---

## 5. Notes / Message Board

Replaces the sticky notes and scribbled reminders on the fridge.

- **Pinned notes** — persistent messages visible on dashboard ("Plumber coming Thursday")
- **Family messages** — "Mom: I'll be home late, leftovers in the fridge"
- **Auto-expire option** — note disappears after its date passes
- **Color/priority flags** — urgent (red), info (blue), fun (green)

---

## 6. Meal Planning (Phase 2)

- **Weekly meal grid** — assign meals to days (dinner focus, optionally lunch/breakfast)
- **Recipe links** — attach a URL or note to each meal
- **Generate grocery items** — tap to add a meal's ingredients to the grocery list
- **Family favorites** — saved meals that can be quickly scheduled again
- **"What's for dinner?"** — prominent display on dashboard for the day's planned meal

---

## 7. Technical Architecture

### Stack
- **Backend:** Go (HTTP server, REST API, WebSocket for real-time updates)
- **Database:** SQLite (single-file, easy backup, sufficient for household scale)
- **Frontend (touchscreen):** Go templates + htmx + Alpine.js + Tailwind CSS + DaisyUI (component library — large, fun, touch-friendly components with built-in theming)
- **Frontend (mobile companion):** Same web app, responsive — or Svelte if interactivity demands it
- **Real-time sync:** WebSocket push so all connected screens/devices update instantly when a change is made
- **Calendar sync:** CalDAV server built-in (or CalDAV client syncing to external providers)

### Deployment
- **Self-hosted** on home server, Raspberry Pi, NAS, or any device that runs Go or Docker
- **Single binary** — Go binary with embedded static assets, zero external dependencies
- **Docker option** — for easy deployment alongside other services
- **Local backups** — SQLite DB snapshot to configurable local path (NAS, USB, etc.)
- **Cloud backups (paid tier)** — encrypted offsite backup via Litestream or scheduled upload to managed storage
- **Remote access (paid tier)** — tunnel relay service (Cloudflare Tunnel or custom) so the app is reachable from outside the home without port forwarding or dynamic DNS
- **Hosted option (paid tier)** — fully managed instance for families who don't want to run hardware
- **HTTPS** — via reverse proxy or built-in Let's Encrypt

### Touchscreen Considerations
- **Large tap targets** — minimum 48px, ideally 64px+ for kitchen use (wet/floury hands)
- **High contrast** — readable from across the kitchen
- **No hover states** — everything works on tap/touch only
- **Swipe gestures** — swipe between days, swipe to complete chores
- **Kiosk mode friendly** — designed to run full-screen in a browser on a dedicated device
- **Auto-brightness** — dim at night, bright during the day (CSS media query or JS-based schedule)
- **Screen burn-in prevention** — subtle pixel shifting or idle mode transition

### Authentication & Multi-Tenancy
- **PIN per family member** — for "who's adding this?" attribution on the kitchen screen
- **Auth accounts separate from family members** — parents/adults have login accounts (email-based, passwordless magic links), kids remain lightweight profiles
- **Multi-tenant support** — multiple households on a single instance with isolated data (required for hosted tier, useful for self-hosted families sharing an instance)
- **LAN kiosk mode** — authenticated session on the kitchen screen, no repeated logins for household members walking past
- **Remote access** — magic link login for secure access over the internet (cloud tier provides the tunnel, auth is built into the core app)
- See `dev/auth-plan.md` for detailed implementation plan

---

## 8. Data Model (Simplified)

```
family_members
  id, name, color, avatar, pin, sort_order

calendar_events
  id, title, description, start_time, end_time, all_day,
  family_member_id, recurrence_rule, location, created_at

chores
  id, title, description, area, points, recurrence_rule,
  assigned_to, rotation_members, created_at

chore_completions
  id, chore_id, completed_by, completed_at

grocery_items
  id, list_id, name, quantity, unit, notes, category,
  checked, checked_by, added_by, created_at

grocery_lists
  id, name, sort_order

notes
  id, title, body, author_id, pinned, priority,
  expires_at, created_at

meals (Phase 2)
  id, date, meal_type, title, recipe_url, notes
```

---

## 9. Development Phases

### Phase 1: MVP — The Fridge Door Replacement
- Dashboard with clock/date/weather
- Family member management
- Calendar with day/week view, basic CRUD, recurring events
- Chore list with assignment, completion, and recurring schedules
- Grocery list with categories, add/check-off
- Responsive layout for touchscreen + mobile
- WebSocket real-time sync

### Phase 2: Quality of Life
- CalDAV sync (Google Calendar, iCloud)
- Meal planning
- Rewards/points system
- Notes/message board
- Frequent grocery items and auto-categorization
- Idle/screensaver mode

### Phase 3: Authentication & Multi-Tenancy
- Passwordless auth (magic link emails)
- Multi-household data isolation
- Household-scoped WebSocket broadcasts
- Session management and admin roles
- Invite flow for adding household members
- See `dev/auth-plan.md` for detailed implementation

### Phase 4: Cloud Services (Paid Tier)
- Remote access tunnel relay (Cloudflare Tunnel integration)
- Encrypted offsite backups (Litestream or scheduled upload)
- Push notifications (mobile companion)
- Account management and billing
- Hosted tier infrastructure

### Phase 5: Polish & Extended Features
- Chore rotation automation
- Grocery suggestions based on purchase history
- Multi-list support (Costco, hardware store, etc.)
- Widgets/integrations (weather detail, school calendar import)
- Kiosk mode installer script for Raspberry Pi
- Local backup/restore tooling

---

## 10. Design Principles

1. **Glanceable** — the most important info is visible from 6 feet away
2. **Tap-first** — every interaction works with a single tap or short swipe
3. **Family-friendly** — a 7-year-old and a 70-year-old should both find it intuitive
4. **Fast** — page transitions under 100ms, no loading spinners for common actions
5. **Offline-resilient** — if the internet drops, the kitchen screen keeps working
6. **Local-first** — your family's data lives on your hardware, cloud services are opt-in
7. **Free forever core** — the self-hosted app is fully functional without a subscription or internet connection
8. **Simple over clever** — no AI magic, no social features, no gamification bloat unless the family opts in
9. **No lock-in** — if the service shuts down, the software keeps running on your hardware