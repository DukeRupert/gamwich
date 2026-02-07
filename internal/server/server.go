package server

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/dukerupert/gamwich/internal/backup"
	"github.com/dukerupert/gamwich/internal/email"
	"github.com/dukerupert/gamwich/internal/handler"
	"github.com/dukerupert/gamwich/internal/license"
	"github.com/dukerupert/gamwich/internal/middleware"
	"github.com/dukerupert/gamwich/internal/push"
	"github.com/dukerupert/gamwich/internal/store"
	"github.com/dukerupert/gamwich/internal/tunnel"
	"github.com/dukerupert/gamwich/internal/weather"
	ws "github.com/dukerupert/gamwich/internal/websocket"
)

type Server struct {
	db              *sql.DB
	hub             *ws.Hub
	familyMemberH   *handler.FamilyMemberHandler
	calendarEventH  *handler.CalendarEventHandler
	choreH          *handler.ChoreHandler
	groceryH        *handler.GroceryHandler
	noteH           *handler.NoteHandler
	rewardH         *handler.RewardHandler
	settingsH       *handler.SettingsHandler
	templateHandler *handler.TemplateHandler
	authH           *handler.AuthHandler
	pushH           *handler.PushHandler
	sessionStore    *store.SessionStore
	householdStore  *store.HouseholdStore
	pushStore       *store.PushStore
	rateLimiter     *middleware.RateLimiter
	licenseClient   *license.Client
	tunnelManager   *tunnel.Manager
	backupManager   *backup.Manager
	pushService     *push.Service
	pushScheduler   *push.Scheduler
	logger          *slog.Logger
}

func New(db *sql.DB, weatherSvc *weather.Service, emailClient *email.Client, baseURL string, licenseClient *license.Client, port string, backupCfg backup.Config, pushCfg push.Config, logger *slog.Logger) *Server {
	hub := ws.NewHub(logger.With("component", "websocket"))

	familyMemberStore := store.NewFamilyMemberStore(db)
	eventStore := store.NewEventStore(db)
	choreStore := store.NewChoreStore(db)
	groceryStore := store.NewGroceryStore(db)
	noteStore := store.NewNoteStore(db)
	rewardStore := store.NewRewardStore(db)
	settingsStore := store.NewSettingsStore(db)

	// Auth stores
	userStore := store.NewUserStore(db)
	householdStore := store.NewHouseholdStore(db)
	sessionStore := store.NewSessionStore(db)
	magicLinkStore := store.NewMagicLinkStore(db)

	backupLogger := logger.With("component", "backup")
	tunnelLogger := logger.With("component", "tunnel")
	pushLogger := logger.With("component", "push")

	// Backup store + manager
	backupStore := store.NewBackupStore(db)
	backupMgr := backup.NewManager(backupCfg, db, backupStore, settingsStore, func(s backup.Status) {
		hub.Broadcast(ws.Message{
			Type:   "backup_status",
			Entity: "backup",
			Action: string(s.State),
			Extra: map[string]any{
				"in_progress": s.InProgress,
				"error":       s.Error,
			},
		})
	}, backupLogger)

	// Tunnel manager
	tunnelCfg := tunnel.Config{
		LocalURL: "http://localhost:" + port,
	}
	if ts, err := settingsStore.GetTunnelSettings(); err == nil {
		tunnelCfg.Token = ts["tunnel_token"]
		tunnelCfg.Enabled = ts["tunnel_enabled"] == "true"
	}
	tunnelMgr := tunnel.NewManager(tunnelCfg, func(s tunnel.Status) {
		hub.Broadcast(ws.Message{
			Type:   "tunnel_status",
			Entity: "tunnel",
			Action: string(s.State),
			Extra: map[string]any{
				"subdomain": s.Subdomain,
				"error":     s.Error,
			},
		})
	}, tunnelLogger)

	// Push notification service + scheduler
	pushSt := store.NewPushStore(db)
	var pushSvc *push.Service
	var pushSched *push.Scheduler
	var pushH *handler.PushHandler
	if pushCfg.VAPIDPublicKey != "" && pushCfg.VAPIDPrivateKey != "" {
		pushSvc = push.NewService(pushCfg.VAPIDPublicKey, pushCfg.VAPIDPrivateKey)
		pushSched = push.NewScheduler(pushSvc, pushSt, eventStore, choreStore, familyMemberStore, pushLogger)
		pushH = handler.NewPushHandler(pushSt, pushSvc, logger.With("component", "push_handler"))
	}

	return &Server{
		db:              db,
		hub:             hub,
		familyMemberH:   handler.NewFamilyMemberHandler(familyMemberStore, hub, logger.With("component", "family_member")),
		calendarEventH:  handler.NewCalendarEventHandler(eventStore, familyMemberStore, hub, logger.With("component", "calendar")),
		choreH:          handler.NewChoreHandler(choreStore, familyMemberStore, hub, logger.With("component", "chore")),
		groceryH:        handler.NewGroceryHandler(groceryStore, familyMemberStore, hub, logger.With("component", "grocery")),
		noteH:           handler.NewNoteHandler(noteStore, familyMemberStore, hub, logger.With("component", "note")),
		rewardH:         handler.NewRewardHandler(rewardStore, familyMemberStore, hub, logger.With("component", "reward")),
		settingsH:       handler.NewSettingsHandler(settingsStore, weatherSvc, backupMgr, hub),
		templateHandler: handler.NewTemplateHandler(familyMemberStore, eventStore, choreStore, groceryStore, noteStore, rewardStore, settingsStore, weatherSvc, hub, licenseClient, tunnelMgr, backupMgr, backupStore, pushSt, pushSvc, pushSched, logger.With("component", "template")),
		authH:           handler.NewAuthHandler(userStore, householdStore, sessionStore, magicLinkStore, emailClient, baseURL, logger.With("component", "auth")),
		pushH:           pushH,
		sessionStore:    sessionStore,
		householdStore:  householdStore,
		pushStore:       pushSt,
		rateLimiter:     middleware.NewRateLimiter(),
		licenseClient:   licenseClient,
		tunnelManager:   tunnelMgr,
		backupManager:   backupMgr,
		pushService:     pushSvc,
		pushScheduler:   pushSched,
		logger:          logger,
	}
}

// SessionStore returns the session store for cleanup tasks.
func (s *Server) SessionStore() *store.SessionStore {
	return s.sessionStore
}

// MagicLinkStore returns the magic link store for cleanup tasks.
func (s *Server) MagicLinkStore() *store.MagicLinkStore {
	return store.NewMagicLinkStore(s.db)
}

// RateLimiter returns the rate limiter for cleanup tasks.
func (s *Server) RateLimiter() *middleware.RateLimiter {
	return s.rateLimiter
}

// LicenseClient returns the license client.
func (s *Server) LicenseClient() *license.Client {
	return s.licenseClient
}

// TunnelManager returns the tunnel manager.
func (s *Server) TunnelManager() *tunnel.Manager {
	return s.tunnelManager
}

// BackupManager returns the backup manager.
func (s *Server) BackupManager() *backup.Manager {
	return s.backupManager
}

// PushScheduler returns the push notification scheduler.
func (s *Server) PushScheduler() *push.Scheduler {
	return s.pushScheduler
}

// PushStore returns the push store for cleanup tasks.
func (s *Server) PushStore() *store.PushStore {
	return s.pushStore
}

func (s *Server) Router() http.Handler {
	outerMux := http.NewServeMux()

	// Public routes (no auth required)
	outerMux.HandleFunc("GET /login", s.authH.LoginPage)
	outerMux.HandleFunc("POST /login", s.rateLimitedHandler(s.authH.Login))
	outerMux.HandleFunc("GET /register", s.authH.RegisterPage)
	outerMux.HandleFunc("POST /register", s.rateLimitedHandler(s.authH.Register))
	outerMux.HandleFunc("GET /auth/verify", s.authH.Verify)
	outerMux.HandleFunc("GET /invite/accept", s.authH.InviteAccept)
	outerMux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	outerMux.HandleFunc("GET /health", s.healthHandler)

	// Protected routes — wrapped with RequireAuth middleware
	protectedMux := http.NewServeMux()
	s.registerProtectedRoutes(protectedMux)

	authMiddleware := middleware.RequireAuth(s.sessionStore, s.householdStore)
	outerMux.Handle("/", authMiddleware(protectedMux))

	// Apply request logging middleware
	return middleware.RequestLogger(s.logger.With("component", "http"))(outerMux)
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) rateLimitedHandler(h http.HandlerFunc) http.HandlerFunc {
	keyFunc := func(r *http.Request) string {
		return middleware.RealIP(r)
	}
	rl := middleware.RateLimit(s.rateLimiter, keyFunc, 10, time.Minute)
	return func(w http.ResponseWriter, r *http.Request) {
		rl(http.HandlerFunc(h)).ServeHTTP(w, r)
	}
}

func (s *Server) registerProtectedRoutes(mux *http.ServeMux) {
	// Auth routes that require authentication
	mux.HandleFunc("POST /logout", s.authH.Logout)
	mux.HandleFunc("GET /households", s.authH.HouseholdsPage)
	mux.HandleFunc("POST /households/switch", s.authH.SwitchHousehold)
	mux.HandleFunc("POST /invite", s.authH.Invite)

	// API routes
	mux.HandleFunc("GET /api/family-members", s.familyMemberH.List)
	mux.HandleFunc("POST /api/family-members", s.familyMemberH.Create)
	mux.HandleFunc("PUT /api/family-members/{id}", s.familyMemberH.Update)
	mux.HandleFunc("DELETE /api/family-members/{id}", s.familyMemberH.Delete)
	mux.HandleFunc("PUT /api/family-members/sort", s.familyMemberH.UpdateSortOrder)

	// PIN routes
	mux.HandleFunc("POST /api/family-members/{id}/pin", s.familyMemberH.SetPIN)
	mux.HandleFunc("DELETE /api/family-members/{id}/pin", s.familyMemberH.ClearPIN)
	mux.HandleFunc("POST /api/family-members/{id}/pin/verify", s.familyMemberH.VerifyPIN)

	// Calendar event API routes
	mux.HandleFunc("POST /api/events", s.calendarEventH.Create)
	mux.HandleFunc("GET /api/events", s.calendarEventH.List)
	mux.HandleFunc("GET /api/events/{id}", s.calendarEventH.Get)
	mux.HandleFunc("PUT /api/events/{id}", s.calendarEventH.Update)
	mux.HandleFunc("DELETE /api/events/{id}", s.calendarEventH.Delete)

	// Chore API routes
	mux.HandleFunc("POST /api/chores", s.choreH.Create)
	mux.HandleFunc("GET /api/chores", s.choreH.List)
	mux.HandleFunc("PUT /api/chores/{id}", s.choreH.Update)
	mux.HandleFunc("DELETE /api/chores/{id}", s.choreH.Delete)
	mux.HandleFunc("POST /api/chores/{id}/complete", s.choreH.Complete)
	mux.HandleFunc("DELETE /api/chores/{id}/completions/{completion_id}", s.choreH.UndoComplete)

	// Grocery API routes
	mux.HandleFunc("POST /api/grocery-lists/{list_id}/items", s.groceryH.CreateItem)
	mux.HandleFunc("GET /api/grocery-lists/{list_id}/items", s.groceryH.ListItems)
	mux.HandleFunc("PUT /api/grocery-lists/{list_id}/items/{id}", s.groceryH.UpdateItem)
	mux.HandleFunc("DELETE /api/grocery-lists/{list_id}/items/{id}", s.groceryH.DeleteItem)
	mux.HandleFunc("POST /api/grocery-lists/{list_id}/items/{id}/check", s.groceryH.ToggleChecked)
	mux.HandleFunc("POST /api/grocery-lists/{list_id}/clear-checked", s.groceryH.ClearChecked)

	// Notes API routes
	mux.HandleFunc("POST /api/notes", s.noteH.Create)
	mux.HandleFunc("GET /api/notes", s.noteH.List)
	mux.HandleFunc("PUT /api/notes/{id}", s.noteH.Update)
	mux.HandleFunc("DELETE /api/notes/{id}", s.noteH.Delete)
	mux.HandleFunc("POST /api/notes/{id}/pin", s.noteH.TogglePinned)

	// Rewards API routes
	mux.HandleFunc("POST /api/rewards", s.rewardH.Create)
	mux.HandleFunc("GET /api/rewards", s.rewardH.List)
	mux.HandleFunc("PUT /api/rewards/{id}", s.rewardH.Update)
	mux.HandleFunc("DELETE /api/rewards/{id}", s.rewardH.Delete)
	mux.HandleFunc("POST /api/rewards/{id}/redeem", s.rewardH.Redeem)
	mux.HandleFunc("GET /api/family-members/{id}/points", s.rewardH.GetPointBalance)
	mux.HandleFunc("GET /api/leaderboard", s.rewardH.GetLeaderboard)

	// Settings API routes
	mux.HandleFunc("GET /api/settings/kiosk", s.settingsH.GetKiosk)
	mux.HandleFunc("PUT /api/settings/kiosk", s.settingsH.UpdateKiosk)
	mux.HandleFunc("GET /api/settings/weather", s.settingsH.GetWeather)
	mux.HandleFunc("PUT /api/settings/weather", s.settingsH.UpdateWeather)
	mux.HandleFunc("GET /api/settings/s3", s.settingsH.GetS3)
	mux.HandleFunc("PUT /api/settings/s3", s.settingsH.UpdateS3)

	// Push notification API routes
	if s.pushH != nil {
		mux.HandleFunc("POST /api/push/subscribe", s.pushH.Subscribe)
		mux.HandleFunc("DELETE /api/push/subscriptions/{id}", s.pushH.Unsubscribe)
		mux.HandleFunc("GET /api/push/subscriptions", s.pushH.ListSubscriptions)
		mux.HandleFunc("GET /api/push/vapid-key", s.pushH.GetVAPIDKey)
		mux.HandleFunc("GET /api/push/preferences", s.pushH.GetPreferences)
		mux.HandleFunc("PUT /api/push/preferences", s.pushH.UpdatePreferences)
		mux.HandleFunc("POST /api/push/test", s.pushH.TestNotification)
	}

	// Page routes — full layout
	mux.HandleFunc("GET /", s.templateHandler.Dashboard)
	mux.HandleFunc("GET /calendar", s.templateHandler.CalendarPage)
	mux.HandleFunc("GET /chores", s.templateHandler.ChoresPage)
	mux.HandleFunc("GET /chores/manage", s.templateHandler.ChoreManagePage)
	mux.HandleFunc("GET /grocery", s.templateHandler.GroceryPage)
	mux.HandleFunc("GET /notes", s.templateHandler.NotesPage)
	mux.HandleFunc("GET /chores/rewards", s.templateHandler.RewardsPage)
	mux.HandleFunc("GET /settings", s.templateHandler.SectionPage("settings"))
	mux.HandleFunc("GET /settings/family", s.templateHandler.FamilyMembers)

	// Section partials (HTMX)
	mux.HandleFunc("GET /partials/dashboard", s.templateHandler.DashboardPartial)
	mux.HandleFunc("GET /partials/calendar", s.templateHandler.CalendarPartial)
	mux.HandleFunc("GET /partials/chores", s.templateHandler.ChoresPartial)
	mux.HandleFunc("GET /partials/grocery", s.templateHandler.GroceryPartial)
	mux.HandleFunc("GET /partials/grocery/items", s.templateHandler.GroceryItemList)
	mux.HandleFunc("POST /partials/grocery/items", s.templateHandler.GroceryItemAdd)
	mux.HandleFunc("POST /partials/grocery/items/{id}/check", s.templateHandler.GroceryItemToggle)
	mux.HandleFunc("DELETE /partials/grocery/items/{id}", s.templateHandler.GroceryItemDelete)
	mux.HandleFunc("POST /partials/grocery/clear-checked", s.templateHandler.GroceryClearChecked)
	mux.HandleFunc("GET /partials/grocery/items/{id}/edit", s.templateHandler.GroceryItemEditForm)
	mux.HandleFunc("PUT /partials/grocery/items/{id}", s.templateHandler.GroceryItemUpdate)
	// Notes partials (HTMX)
	mux.HandleFunc("GET /partials/notes", s.templateHandler.NotesPartial)
	mux.HandleFunc("GET /partials/notes/list", s.templateHandler.NoteList)
	mux.HandleFunc("GET /partials/notes/new", s.templateHandler.NoteNewForm)
	mux.HandleFunc("GET /partials/notes/{id}/edit", s.templateHandler.NoteEditForm)
	mux.HandleFunc("POST /partials/notes", s.templateHandler.NoteCreate)
	mux.HandleFunc("PUT /partials/notes/{id}", s.templateHandler.NoteUpdate)
	mux.HandleFunc("DELETE /partials/notes/{id}", s.templateHandler.NoteDelete)
	mux.HandleFunc("POST /partials/notes/{id}/pin", s.templateHandler.NoteTogglePin)

	// Rewards partials (HTMX)
	mux.HandleFunc("GET /partials/rewards", s.templateHandler.RewardsPartial)
	mux.HandleFunc("GET /partials/rewards/list", s.templateHandler.RewardsList)
	mux.HandleFunc("POST /partials/rewards/{id}/redeem", s.templateHandler.RewardRedeem)

	// Reward management partials (settings)
	mux.HandleFunc("GET /partials/settings/rewards", s.templateHandler.RewardManagePartial)
	mux.HandleFunc("GET /partials/settings/rewards/new", s.templateHandler.RewardNewForm)
	mux.HandleFunc("GET /partials/settings/rewards/{id}/edit", s.templateHandler.RewardEditForm)
	mux.HandleFunc("POST /partials/settings/rewards", s.templateHandler.RewardCreate)
	mux.HandleFunc("PUT /partials/settings/rewards/{id}", s.templateHandler.RewardUpdate)
	mux.HandleFunc("DELETE /partials/settings/rewards/{id}", s.templateHandler.RewardDelete)

	mux.HandleFunc("GET /partials/settings", s.templateHandler.SettingsPartial)
	mux.HandleFunc("GET /partials/settings/kiosk", s.templateHandler.KioskSettingsPartial)
	mux.HandleFunc("PUT /partials/settings/kiosk", s.templateHandler.KioskSettingsUpdate)
	mux.HandleFunc("GET /partials/settings/weather", s.templateHandler.WeatherSettingsPartial)
	mux.HandleFunc("PUT /partials/settings/weather", s.templateHandler.WeatherSettingsUpdate)
	mux.HandleFunc("GET /partials/settings/theme", s.templateHandler.ThemeSettingsPartial)
	mux.HandleFunc("PUT /partials/settings/theme", s.templateHandler.ThemeSettingsUpdate)
	mux.HandleFunc("GET /partials/settings/license", s.templateHandler.LicenseSettingsPartial)
	mux.HandleFunc("PUT /partials/settings/license", s.templateHandler.LicenseKeyUpdate)
	mux.HandleFunc("GET /partials/settings/s3", s.templateHandler.S3SettingsPartial)
	mux.HandleFunc("PUT /partials/settings/s3", s.templateHandler.S3SettingsUpdate)
	mux.HandleFunc("GET /partials/settings/tunnel", s.templateHandler.TunnelSettingsPartial)
	mux.HandleFunc("PUT /partials/settings/tunnel", s.templateHandler.TunnelSettingsUpdate)
	mux.HandleFunc("GET /partials/settings/tunnel/status", s.templateHandler.TunnelStatusPartial)
	mux.HandleFunc("GET /partials/idle/next-event", s.templateHandler.NextUpcomingEventPartial)

	// Calendar view partials
	mux.HandleFunc("GET /partials/calendar/day", s.templateHandler.CalendarDayPartial)
	mux.HandleFunc("GET /partials/calendar/week", s.templateHandler.CalendarWeekPartial)
	mux.HandleFunc("GET /partials/calendar/events/new", s.templateHandler.CalendarEventNewForm)
	mux.HandleFunc("GET /partials/calendar/events/{id}", s.templateHandler.CalendarEventDetail)
	mux.HandleFunc("GET /partials/calendar/events/{id}/edit", s.templateHandler.CalendarEventEditForm)
	mux.HandleFunc("POST /partials/calendar/events", s.templateHandler.CalendarEventCreate)
	mux.HandleFunc("PUT /partials/calendar/events/{id}", s.templateHandler.CalendarEventUpdate)
	mux.HandleFunc("DELETE /partials/calendar/events/{id}", s.templateHandler.CalendarEventDeleteForm)
	mux.HandleFunc("GET /partials/calendar/events/{id}/recurrence-edit", s.templateHandler.RecurrenceEditChoice)
	mux.HandleFunc("GET /partials/calendar/events/{id}/recurrence-delete", s.templateHandler.RecurrenceDeleteChoice)

	// Chore partials (HTMX)
	mux.HandleFunc("GET /partials/chores/by-person", s.templateHandler.ChoreListByPerson)
	mux.HandleFunc("GET /partials/chores/by-area", s.templateHandler.ChoreListByArea)
	mux.HandleFunc("GET /partials/chores/all", s.templateHandler.ChoreListAll)
	mux.HandleFunc("GET /partials/chores/new", s.templateHandler.ChoreNewForm)
	mux.HandleFunc("GET /partials/chores/{id}/edit", s.templateHandler.ChoreEditForm)
	mux.HandleFunc("POST /partials/chores", s.templateHandler.ChoreCreate)
	mux.HandleFunc("PUT /partials/chores/{id}", s.templateHandler.ChoreUpdate)
	mux.HandleFunc("DELETE /partials/chores/{id}", s.templateHandler.ChoreDelete)
	mux.HandleFunc("POST /partials/chores/{id}/complete", s.templateHandler.ChoreComplete)
	mux.HandleFunc("POST /partials/chores/{id}/undo-complete", s.templateHandler.ChoreUndoComplete)
	mux.HandleFunc("GET /partials/chores/manage", s.templateHandler.ChoreManagePartial)
	mux.HandleFunc("GET /partials/chores/areas", s.templateHandler.ChoreAreaList)
	mux.HandleFunc("POST /partials/chores/areas", s.templateHandler.ChoreAreaCreate)
	mux.HandleFunc("PUT /partials/chores/areas/{id}", s.templateHandler.ChoreAreaUpdate)
	mux.HandleFunc("DELETE /partials/chores/areas/{id}", s.templateHandler.ChoreAreaDelete)

	// Weather partial (HTMX polling)
	mux.HandleFunc("GET /partials/weather", s.templateHandler.WeatherPartial)

	// User selection partials
	mux.HandleFunc("POST /partials/user/select/{id}", s.templateHandler.SetActiveUser)
	mux.HandleFunc("GET /partials/user/pin-challenge/{id}", s.templateHandler.UserPINChallenge)
	mux.HandleFunc("POST /partials/user/verify-pin/{id}", s.templateHandler.VerifyPINAndSetUser)

	// Family member partials (HTMX)
	mux.HandleFunc("GET /partials/family-members", s.templateHandler.FamilyMemberList)
	mux.HandleFunc("GET /partials/family-members/{id}/edit", s.templateHandler.FamilyMemberEditForm)
	mux.HandleFunc("POST /partials/family-members", s.templateHandler.FamilyMemberCreate)
	mux.HandleFunc("PUT /partials/family-members/{id}", s.templateHandler.FamilyMemberUpdate)
	mux.HandleFunc("DELETE /partials/family-members/{id}", s.templateHandler.FamilyMemberDelete)

	// PIN partials
	mux.HandleFunc("GET /partials/family-members/{id}/pin", s.templateHandler.PINSetupForm)
	mux.HandleFunc("GET /partials/family-members/{id}/pin/gate", s.templateHandler.PINGate)
	mux.HandleFunc("POST /partials/family-members/{id}/pin/verify", s.templateHandler.PINVerifyThenAct)
	mux.HandleFunc("POST /partials/family-members/{id}/pin", s.templateHandler.PINSetup)

	// Backup partials (HTMX)
	mux.HandleFunc("GET /partials/settings/backup", s.templateHandler.BackupSettingsPartial)
	mux.HandleFunc("PUT /partials/settings/backup", s.templateHandler.BackupSettingsUpdate)
	mux.HandleFunc("PUT /partials/settings/backup/passphrase", s.templateHandler.BackupPassphraseUpdate)
	mux.HandleFunc("POST /partials/settings/backup/now", s.templateHandler.BackupNow)
	mux.HandleFunc("GET /partials/settings/backup/history", s.templateHandler.BackupHistoryPartial)
	mux.HandleFunc("GET /partials/settings/backup/status", s.templateHandler.BackupStatusPartial)
	mux.HandleFunc("POST /partials/settings/backup/restore/{id}", s.templateHandler.BackupRestore)
	mux.HandleFunc("GET /partials/settings/backup/download/{id}", s.templateHandler.BackupDownload)

	// Push notification partials (HTMX)
	mux.HandleFunc("GET /partials/settings/push", s.templateHandler.PushSettingsPartial)
	mux.HandleFunc("PUT /partials/settings/push/preferences", s.templateHandler.PushPreferencesUpdate)
	mux.HandleFunc("GET /partials/settings/push/devices", s.templateHandler.PushDevicesList)
	mux.HandleFunc("DELETE /partials/settings/push/devices/{id}", s.templateHandler.PushDeviceDelete)

	// WebSocket
	mux.HandleFunc("GET /ws", ws.HandleWebSocket(s.hub))
}
