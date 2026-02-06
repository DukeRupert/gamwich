# Gamwich — Business Plan

> An open-source family dashboard with paid cloud services. Free on your own hardware, subscription for remote access and backups.

---

## Market Opportunity

### The Problem

Families need a shared, glanceable place to coordinate schedules, chores, groceries, and messages. The refrigerator door has served this role for decades — but paper calendars, sticky notes, and scribbled grocery lists don't sync to phones, can't send reminders, and get lost in clutter.

Existing digital solutions force families into one of two bad tradeoffs:

1. **Expensive proprietary hardware** (Skylight, Hearth Display) — $300 upfront + $79/year subscription, vendor lock-in, device becomes a paperweight if the company shuts down
2. **Fragmented free apps** (Google Calendar, Cozi, shared iPhone reminders) — no unified dashboard, no kiosk/touchscreen mode, family data scattered across services controlled by ad companies
3. **DIY tinkerer tools** (Home Assistant dashboards, MagicMirror, DAKboard) — powerful but require significant technical setup, YAML configuration, and ongoing maintenance

No product today offers: a polished, purpose-built family dashboard that's free to self-host, runs on any hardware, and optionally connects to paid cloud services for remote access and backup.

### Target Market

**Primary: Tech-comfortable parents (self-hosted tier)**
Parents who are comfortable running a Docker container or flashing a Raspberry Pi. They value privacy, ownership, and not paying subscriptions for basic functionality. This audience actively seeks self-hosted alternatives (Home Assistant, Nextcloud, Immich communities). They're the early adopters and evangelists.

**Secondary: Non-technical families (hosted tier)**
Parents who want the Skylight experience without the Skylight price. They don't want to manage hardware — they want to open a browser on a spare tablet and have it work. The hosted tier serves this audience at a lower price than Skylight with no proprietary hardware requirement.

**Tertiary: Skylight/Hearth upgraders**
Families who already bought a Skylight Calendar and are frustrated by the subscription, limited features, or sync limitations. Gamwich on a $50 Fire tablet offers more functionality at lower total cost.

---

## Competitive Landscape

### Direct Competitors

| Product | Hardware Cost | Subscription | Key Limitations |
|---------|-------------|--------------|-----------------|
| **Skylight Calendar** | $300 | $79/year (Plus) | Proprietary hardware, Google-only full sync, no offline, vendor lock-in |
| **Hearth Display** | $300+ | Included (for now) | Proprietary hardware, early stage, limited ecosystem |
| **DAKboard** | BYO | $5/month | Display-focused (calendar/photos/widgets), not a household management tool |

### Indirect Competitors

| Product | Cost | Key Limitations |
|---------|------|-----------------|
| **Cozi** | Free (ads) / $30/year | Phone app only, no kiosk/dashboard mode, ad-supported |
| **Google Calendar (shared)** | Free | Not a household tool — no chores, groceries, rewards |
| **MagicMirror** | BYO + free | Tinkerer tool, requires significant setup, display-only |
| **Home Assistant** | BYO + free | General-purpose home automation, family dashboard is a DIY project |
| **Mango Display** | BYO + free tier | Display widgets only, limited household management features |

### Gamwich's Position

Gamwich occupies the gap between "expensive proprietary device" and "DIY tinkerer project." It's a **purpose-built household management app** (not just a display widget engine) that's **free to self-host** and **polished enough for non-technical family members** to use.

The closest analogy is **Plex** (for media) or **Home Assistant + Nabu Casa** (for home automation): an open-source core that works locally, with optional paid services for remote access and cloud features.

---

## Product Tiers & Pricing

### Free Tier (Self-Hosted)

**Price:** $0, forever.

**What's included:**
- Full app: dashboard, calendar, chores, groceries, notes, meal planning, rewards
- Multi-household support
- WebSocket real-time sync across devices on the local network
- PWA / kiosk mode
- Local backups (SQLite file copy to configurable path)
- All future core features

**Deployment:** Single Go binary or Docker container on user's hardware.

**Why free:** The free tier is the growth engine. It builds community, drives word-of-mouth, and establishes trust. Users who love the free product are the ones who convert to paid when they want remote access.

---

### Cloud Tier

**Price:** ~$5/month ($50/year)

**What's included (everything in Free, plus):**
- **Remote access tunnel** — access your home Gamwich instance from anywhere via a managed tunnel relay (no port forwarding, no dynamic DNS, no VPN setup)
- **Encrypted offsite backups** — daily encrypted backup of your SQLite database to managed cloud storage, with configurable retention
- **Push notifications** — mobile push for calendar reminders, chore due dates, grocery list updates
- **Restore from backup** — one-click restore from any backup snapshot
- **Priority support**

**How it works:** The Gamwich instance on your hardware connects outbound to the Gamwich relay service. The relay proxies HTTPS traffic to your instance. Your data still lives on your hardware — the relay only forwards encrypted traffic. Backups are encrypted client-side before upload.

**Infrastructure cost per customer:** Minimal — tunnel relay bandwidth (~pennies/month), S3 storage for a small SQLite file (~$0.01/month), push notification delivery (~$0.001/notification). Gross margin >90%.

---

### Hosted Tier

**Price:** ~$10/month ($100/year)

**What's included (everything in Cloud, plus):**
- **Fully managed instance** — we run the Gamwich server, customer just opens a browser
- **Custom subdomain** — `yourfamily.gamwich.app`
- **Automatic updates** — always on the latest version
- **Zero setup** — sign up, create household, open on a tablet, done

**How it works:** Multi-tenant Gamwich instance running on managed infrastructure. Each household's data is isolated. May use PostgreSQL at scale for the hosted platform while the self-hosted product stays on SQLite.

**Infrastructure cost per customer:** Small VM or container slice (~$2-3/month at scale). Gross margin ~70-80%.

---

### Pricing Rationale

| | Skylight (comparable) | Gamwich Cloud | Gamwich Hosted |
|---|---|---|---|
| Year 1 cost | $379 ($300 hardware + $79 sub) | $50 (BYO hardware) | $100 (BYO hardware) |
| Year 2+ cost | $79/year | $50/year | $100/year |
| 3-year total | $537 | $150 | $300 |

Gamwich Cloud is **72% cheaper than Skylight over 3 years**, and the family keeps a fully functional free app if they ever cancel.

---

## Go-To-Market Strategy

### Phase 1: Community & Early Adopters

**Channel:** Self-hosted / privacy communities
- Reddit: r/selfhosted, r/homelab, r/homeassistant, r/raspberry_pi, r/privacytoolsIO
- Hacker News, Lobste.rs
- Self-hosted YouTube channels and blogs

**Action:**
- Open-source the core app on GitHub
- Write a launch post: "I built a free Skylight Calendar alternative that runs on a Raspberry Pi"
- Provide excellent documentation and a one-line Docker install
- Build community via GitHub Discussions or Discord

**Goal:** 500-1,000 self-hosted installations, active community contributing feedback and bug reports.

### Phase 2: Cloud Launch & Conversion

**Channel:** Existing free users + broader reach
- In-app prompt: "Access Gamwich from work? Try Cloud for $5/month"
- Email list from optional registration (never required for free tier)
- Blog content: "How to set up a family dashboard on a Fire Tablet for $50"

**Action:**
- Launch Cloud tier with remote access and backups
- Launch Hosted tier for non-technical families
- Build referral program (invite a family, get a month free)

**Goal:** 5-10% conversion from free to paid. 100 paying customers in first 6 months post-launch.

### Phase 3: Broader Market

**Channel:** Family/parenting communities, product review sites
- Parenting blogs and influencers (the Skylight review ecosystem is large)
- Product Hunt launch
- Comparison content: "Skylight vs. Gamwich: honest comparison"

**Action:**
- Hosted tier is the primary pitch to non-technical families
- Offer a 14-day free trial of Hosted
- Partner with tablet manufacturers or resellers for bundle deals (optional)

**Goal:** Sustainable recurring revenue from Cloud + Hosted subscribers.

---

## Revenue Projections (Conservative)

| Metric | Month 6 | Month 12 | Month 24 |
|--------|---------|----------|----------|
| Free installations | 500 | 2,000 | 5,000 |
| Cloud subscribers | 25 | 100 | 300 |
| Hosted subscribers | 10 | 50 | 150 |
| Monthly revenue | $225 | $1,000 | $3,000 |
| Annual run rate | $2,700 | $12,000 | $36,000 |

These are conservative solo-founder numbers. The business is viable as a side project at $12K ARR and becomes a sustainable small business at $36K+ ARR.

---

## Technical Architecture for Paid Tiers

### Remote Access (Cloud Tier)

**Option A: Cloudflare Tunnel integration (recommended for launch)**
- Gamwich binary includes a Cloudflare Tunnel client (`cloudflared` library or subprocess)
- On Cloud tier activation, the instance registers with our Cloudflare account
- Each household gets a subdomain: `abc123.tunnel.gamwich.app`
- Cloudflare handles TLS, DDoS protection, and global edge routing
- Low operational burden, battle-tested infrastructure

**Option B: Custom WebSocket relay (future consideration)**
- Gamwich instance opens outbound WebSocket to `relay.gamwich.app`
- Relay service proxies inbound HTTPS requests through the WebSocket
- More control, more operational burden, but no third-party dependency

### Encrypted Backups (Cloud Tier)

- Built-in backup agent in the Gamwich binary (enabled on Cloud tier activation)
- Flow: checkpoint SQLite WAL → copy database → encrypt with household-specific key (derived from a secret the user holds) → upload to S3-compatible storage
- Schedule: daily, configurable retention (30 days default)
- Restore: download → decrypt → replace database → restart
- Alternative: integrate Litestream for continuous replication

### Hosted Tier Infrastructure

- Multi-tenant deployment on managed infrastructure (Fly.io, Railway, or bare VPS)
- Each household runs as an isolated container or process with its own SQLite database
- Shared reverse proxy routes `*.gamwich.app` subdomains to the correct instance
- Automated provisioning: sign-up → create container → assign subdomain → ready in <30 seconds
- At scale (1,000+ hosted households), consider a shared PostgreSQL backend for the hosted platform only

### Billing & Account Management

- Stripe for payment processing
- Account portal: manage subscription, view invoices, download backups, manage tunnel
- License key system: Cloud tier activation sends a key to the self-hosted instance, which enables tunnel and backup features
- Graceful degradation: if subscription lapses, tunnel and backups stop but the local app continues to work with all data intact

---

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Small addressable market | Medium | High | Keep costs low (solo founder, minimal infrastructure). The business is viable at small scale. |
| Low free-to-paid conversion | Medium | Medium | Ensure the free product is genuinely great — conversion follows from love, not gates. Remote access is a natural desire for working parents. |
| Skylight drops price / goes free | Low | Medium | Skylight's business model requires hardware margins. They can't match "free on your own hardware." |
| Open source forked with competing cloud | Low | Low | Cloud services require infrastructure and trust. The original project has brand advantage. AGPL or BSL license for the cloud service components if needed. |
| Support burden from self-hosted users | Medium | Medium | Excellent documentation, community forums (not 1:1 support for free tier). Paid tiers get priority support. |
| Technical complexity of tunnel relay | Medium | Medium | Start with Cloudflare Tunnel (off-the-shelf). Only build custom infrastructure if scale demands it. |

---

## Success Metrics

- **Free tier:** GitHub stars, Docker pulls, active installations (opt-in telemetry), community engagement
- **Cloud tier:** conversion rate from free, churn rate, NPS
- **Hosted tier:** sign-up rate, trial-to-paid conversion, churn
- **Product:** daily active households (DAH), features used per household, retention at 30/60/90 days

---

## Summary

Gamwich's thesis: **families will pay for convenience and peace of mind on top of a product they already love and trust**. The free self-hosted tier removes all friction for adoption. The paid tiers monetize the natural desire for remote access and data safety. The total cost is a fraction of Skylight, with no hardware lock-in and no vendor dependency.

The model is proven by Plex, Home Assistant (Nabu Casa), Obsidian, and Bitwarden. The family dashboard market doesn't have this player yet.
