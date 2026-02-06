package server

import (
	"database/sql"
	"net/http"

	"github.com/dukerupert/gamwich/internal/handler"
	"github.com/dukerupert/gamwich/internal/store"
	"github.com/dukerupert/gamwich/internal/weather"
)

type Server struct {
	db              *sql.DB
	familyMemberH   *handler.FamilyMemberHandler
	calendarEventH  *handler.CalendarEventHandler
	templateHandler *handler.TemplateHandler
}

func New(db *sql.DB, weatherSvc *weather.Service) *Server {
	familyMemberStore := store.NewFamilyMemberStore(db)
	eventStore := store.NewEventStore(db)

	return &Server{
		db:              db,
		familyMemberH:   handler.NewFamilyMemberHandler(familyMemberStore),
		calendarEventH:  handler.NewCalendarEventHandler(eventStore, familyMemberStore),
		templateHandler: handler.NewTemplateHandler(familyMemberStore, eventStore, weatherSvc),
	}
}

func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()

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

	// Page routes — full layout
	mux.HandleFunc("GET /", s.templateHandler.Dashboard)
	mux.HandleFunc("GET /calendar", s.templateHandler.SectionPage("calendar"))
	mux.HandleFunc("GET /chores", s.templateHandler.SectionPage("chores"))
	mux.HandleFunc("GET /grocery", s.templateHandler.SectionPage("grocery"))
	mux.HandleFunc("GET /settings", s.templateHandler.SectionPage("settings"))
	mux.HandleFunc("GET /settings/family", s.templateHandler.FamilyMembers)

	// Section partials (HTMX)
	mux.HandleFunc("GET /partials/dashboard", s.templateHandler.DashboardPartial)
	mux.HandleFunc("GET /partials/calendar", s.templateHandler.CalendarPartial)
	mux.HandleFunc("GET /partials/chores", s.templateHandler.ChoresPartial)
	mux.HandleFunc("GET /partials/grocery", s.templateHandler.GroceryPartial)
	mux.HandleFunc("GET /partials/settings", s.templateHandler.SettingsPartial)

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

	// Static files
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	return mux
}
