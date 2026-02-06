package server

import (
	"database/sql"
	"net/http"

	"github.com/dukerupert/gamwich/internal/handler"
	"github.com/dukerupert/gamwich/internal/store"
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
	settingsH       *handler.SettingsHandler
	templateHandler *handler.TemplateHandler
}

func New(db *sql.DB, weatherSvc *weather.Service) *Server {
	hub := ws.NewHub()

	familyMemberStore := store.NewFamilyMemberStore(db)
	eventStore := store.NewEventStore(db)
	choreStore := store.NewChoreStore(db)
	groceryStore := store.NewGroceryStore(db)
	settingsStore := store.NewSettingsStore(db)

	return &Server{
		db:              db,
		hub:             hub,
		familyMemberH:   handler.NewFamilyMemberHandler(familyMemberStore, hub),
		calendarEventH:  handler.NewCalendarEventHandler(eventStore, familyMemberStore, hub),
		choreH:          handler.NewChoreHandler(choreStore, familyMemberStore, hub),
		groceryH:        handler.NewGroceryHandler(groceryStore, familyMemberStore, hub),
		settingsH:       handler.NewSettingsHandler(settingsStore, weatherSvc, hub),
		templateHandler: handler.NewTemplateHandler(familyMemberStore, eventStore, choreStore, groceryStore, settingsStore, weatherSvc, hub),
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

	// Settings API routes
	mux.HandleFunc("GET /api/settings/kiosk", s.settingsH.GetKiosk)
	mux.HandleFunc("PUT /api/settings/kiosk", s.settingsH.UpdateKiosk)
	mux.HandleFunc("GET /api/settings/weather", s.settingsH.GetWeather)
	mux.HandleFunc("PUT /api/settings/weather", s.settingsH.UpdateWeather)

	// Page routes — full layout
	mux.HandleFunc("GET /", s.templateHandler.Dashboard)
	mux.HandleFunc("GET /calendar", s.templateHandler.CalendarPage)
	mux.HandleFunc("GET /chores", s.templateHandler.ChoresPage)
	mux.HandleFunc("GET /chores/manage", s.templateHandler.ChoreManagePage)
	mux.HandleFunc("GET /grocery", s.templateHandler.GroceryPage)
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
	mux.HandleFunc("GET /partials/settings", s.templateHandler.SettingsPartial)
	mux.HandleFunc("GET /partials/settings/kiosk", s.templateHandler.KioskSettingsPartial)
	mux.HandleFunc("PUT /partials/settings/kiosk", s.templateHandler.KioskSettingsUpdate)
	mux.HandleFunc("GET /partials/settings/weather", s.templateHandler.WeatherSettingsPartial)
	mux.HandleFunc("PUT /partials/settings/weather", s.templateHandler.WeatherSettingsUpdate)
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

	// WebSocket
	mux.HandleFunc("GET /ws", ws.HandleWebSocket(s.hub))

	// Static files
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	return mux
}
