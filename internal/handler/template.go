package handler

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dukerupert/gamwich/internal/model"
	"github.com/dukerupert/gamwich/internal/store"
	"github.com/dukerupert/gamwich/internal/weather"
	"golang.org/x/crypto/bcrypt"
)

const activeUserCookie = "gamwich_active_user"

type TemplateHandler struct {
	store      *store.FamilyMemberStore
	eventStore *store.EventStore
	weatherSvc *weather.Service
	templates  *template.Template
}

func NewTemplateHandler(s *store.FamilyMemberStore, es *store.EventStore, w *weather.Service) *TemplateHandler {
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
	}
	tmpl := template.Must(template.New("").Funcs(funcMap).ParseGlob("web/templates/*.html"))
	return &TemplateHandler{
		store:      s,
		eventStore: es,
		weatherSvc: w,
		templates:  tmpl,
	}
}

// buildDashboardData assembles the common data map for layout rendering.
func (h *TemplateHandler) buildDashboardData(r *http.Request, section string) (map[string]any, error) {
	members, err := h.store.List()
	if err != nil {
		return nil, fmt.Errorf("failed to load members: %w", err)
	}

	activeUserID := h.activeUserFromCookie(r)

	data := map[string]any{
		"Title":         "Gamwich",
		"Members":       members,
		"ActiveUserID":  activeUserID,
		"ActiveSection": section,
		"Weather":       h.weatherSvc.GetWeather(),
	}
	return data, nil
}

// activeUserFromCookie reads and parses the active user ID from the cookie.
func (h *TemplateHandler) activeUserFromCookie(r *http.Request) int64 {
	c, err := r.Cookie(activeUserCookie)
	if err != nil {
		return 0
	}
	id, err := strconv.ParseInt(c.Value, 10, 64)
	if err != nil {
		return 0
	}
	return id
}

// renderSection pre-renders a named template to a buffer and returns it as template.HTML.
func (h *TemplateHandler) renderSection(name string, data any) (template.HTML, error) {
	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, name, data); err != nil {
		return "", fmt.Errorf("render section %q: %w", name, err)
	}
	return template.HTML(buf.String()), nil
}

// Dashboard renders the main dashboard page with the home section.
func (h *TemplateHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	data, err := h.buildDashboardData(r, "dashboard")
	if err != nil {
		http.Error(w, "failed to load data", http.StatusInternalServerError)
		return
	}

	content, err := h.renderSection("dashboard-content", data)
	if err != nil {
		log.Printf("render dashboard content: %v", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	data["Content"] = content

	h.render(w, "layout.html", data)
}

// SectionPage returns a handler that renders the full layout with the given section active.
func (h *TemplateHandler) SectionPage(section string) http.HandlerFunc {
	templateName := section + "-content"
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := h.buildDashboardData(r, section)
		if err != nil {
			http.Error(w, "failed to load data", http.StatusInternalServerError)
			return
		}

		content, err := h.renderSection(templateName, data)
		if err != nil {
			log.Printf("render %s content: %v", section, err)
			http.Error(w, "template error", http.StatusInternalServerError)
			return
		}
		data["Content"] = content

		h.render(w, "layout.html", data)
	}
}

// DashboardPartial renders only the dashboard section content for HTMX swap.
func (h *TemplateHandler) DashboardPartial(w http.ResponseWriter, r *http.Request) {
	data, err := h.buildDashboardData(r, "dashboard")
	if err != nil {
		http.Error(w, "failed to load data", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "dashboard-content", data)
}

// CalendarPage renders the full calendar page inside the dashboard layout.
func (h *TemplateHandler) CalendarPage(w http.ResponseWriter, r *http.Request) {
	data, err := h.buildDashboardData(r, "calendar")
	if err != nil {
		http.Error(w, "failed to load data", http.StatusInternalServerError)
		return
	}

	view := r.URL.Query().Get("view")
	if view != "week" {
		view = "day"
	}
	date := h.parseCalendarDate(r)

	viewContent, err := h.buildCalendarViewContent(view, date)
	if err != nil {
		log.Printf("build calendar view: %v", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}

	calendarData := map[string]any{
		"View":        view,
		"DateStr":     date.Format("2006-01-02"),
		"ViewContent": viewContent,
	}

	content, err := h.renderSection("calendar-content", calendarData)
	if err != nil {
		log.Printf("render calendar content: %v", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	data["Content"] = content

	h.render(w, "layout.html", data)
}

// CalendarPartial renders the calendar section with the default or requested view.
func (h *TemplateHandler) CalendarPartial(w http.ResponseWriter, r *http.Request) {
	view := r.URL.Query().Get("view")
	if view != "week" {
		view = "day"
	}
	date := h.parseCalendarDate(r)

	viewContent, err := h.buildCalendarViewContent(view, date)
	if err != nil {
		log.Printf("build calendar view: %v", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}

	h.renderPartial(w, "calendar-content", map[string]any{
		"View":        view,
		"DateStr":     date.Format("2006-01-02"),
		"ViewContent": viewContent,
	})
}

// CalendarDayPartial renders the day view for HTMX swap.
func (h *TemplateHandler) CalendarDayPartial(w http.ResponseWriter, r *http.Request) {
	date := h.parseCalendarDate(r)
	data, err := h.buildDayViewData(date)
	if err != nil {
		log.Printf("build day view: %v", err)
		http.Error(w, "failed to load events", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "calendar-day-view", data)
}

// CalendarWeekPartial renders the week view for HTMX swap.
func (h *TemplateHandler) CalendarWeekPartial(w http.ResponseWriter, r *http.Request) {
	date := h.parseCalendarDate(r)
	data, err := h.buildWeekViewData(date)
	if err != nil {
		log.Printf("build week view: %v", err)
		http.Error(w, "failed to load events", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "calendar-week-view", data)
}

// CalendarEventDetail renders the event detail card in the modal.
func (h *TemplateHandler) CalendarEventDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	event, err := h.eventStore.GetByID(id)
	if err != nil {
		http.Error(w, "failed to get event", http.StatusInternalServerError)
		return
	}
	if event == nil {
		http.Error(w, "event not found", http.StatusNotFound)
		return
	}

	data := h.buildEventDetailData(event)
	h.renderPartial(w, "calendar-event-detail", data)
}

// ChoresPartial renders the chores placeholder for HTMX swap.
func (h *TemplateHandler) ChoresPartial(w http.ResponseWriter, r *http.Request) {
	h.renderPartial(w, "chores-content", nil)
}

// GroceryPartial renders the grocery placeholder for HTMX swap.
func (h *TemplateHandler) GroceryPartial(w http.ResponseWriter, r *http.Request) {
	h.renderPartial(w, "grocery-content", nil)
}

// SettingsPartial renders the settings section for HTMX swap.
func (h *TemplateHandler) SettingsPartial(w http.ResponseWriter, r *http.Request) {
	h.renderPartial(w, "settings-content", nil)
}

// WeatherPartial renders the weather widget for HTMX polling refresh.
func (h *TemplateHandler) WeatherPartial(w http.ResponseWriter, r *http.Request) {
	h.renderPartial(w, "weather-widget", h.weatherSvc.GetWeather())
}

// SetActiveUser sets the active user cookie for non-PIN members.
func (h *TemplateHandler) SetActiveUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	member, err := h.store.GetByID(id)
	if err != nil || member == nil {
		http.Error(w, "member not found", http.StatusNotFound)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     activeUserCookie,
		Value:    strconv.FormatInt(id, 10),
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60, // 30 days
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	members, err := h.store.List()
	if err != nil {
		http.Error(w, "failed to load members", http.StatusInternalServerError)
		return
	}

	h.renderPartial(w, "user-select-bar", map[string]any{
		"Members":      members,
		"ActiveUserID": id,
	})
}

// UserPINChallenge renders the PIN challenge template into the modal body.
func (h *TemplateHandler) UserPINChallenge(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	member, err := h.store.GetByID(id)
	if err != nil || member == nil {
		http.Error(w, "member not found", http.StatusNotFound)
		return
	}

	h.renderPartial(w, "pin-challenge", member)
}

// VerifyPINAndSetUser verifies the PIN and sets the active user cookie on success.
func (h *TemplateHandler) VerifyPINAndSetUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	pin := r.FormValue("pin")

	hash, err := h.store.GetPINHash(id)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(pin)); err != nil {
		member, _ := h.store.GetByID(id)
		h.renderToast(w, "error", "Incorrect PIN")
		h.renderPartial(w, "pin-challenge", member)
		return
	}

	// PIN verified — set cookie and close modal.
	http.SetCookie(w, &http.Cookie{
		Name:     activeUserCookie,
		Value:    strconv.FormatInt(id, 10),
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	// Close the modal via HX-Trigger header.
	w.Header().Set("HX-Trigger", "closeModal")

	members, err := h.store.List()
	if err != nil {
		http.Error(w, "failed to load members", http.StatusInternalServerError)
		return
	}

	h.renderToast(w, "success", "Switched user")
	h.renderPartial(w, "user-select-bar", map[string]any{
		"Members":      members,
		"ActiveUserID": id,
	})
}

// FamilyMembers renders the family members management page inside the dashboard layout.
func (h *TemplateHandler) FamilyMembers(w http.ResponseWriter, r *http.Request) {
	data, err := h.buildDashboardData(r, "settings")
	if err != nil {
		http.Error(w, "failed to load data", http.StatusInternalServerError)
		return
	}

	content, err := h.renderSection("family-members-content", data)
	if err != nil {
		log.Printf("render family members content: %v", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	data["Content"] = content

	h.render(w, "layout.html", data)
}

func (h *TemplateHandler) FamilyMemberList(w http.ResponseWriter, r *http.Request) {
	members, err := h.store.List()
	if err != nil {
		http.Error(w, "failed to load family members", http.StatusInternalServerError)
		return
	}

	h.renderPartial(w, "member-list", map[string]any{"Members": members})
}

func (h *TemplateHandler) FamilyMemberEditForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	member, err := h.store.GetByID(id)
	if err != nil || member == nil {
		http.Error(w, "family member not found", http.StatusNotFound)
		return
	}

	h.renderPartial(w, "member-edit-form", member)
}

func (h *TemplateHandler) FamilyMemberCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	color := r.FormValue("color")
	avatarEmoji := r.FormValue("avatar_emoji")

	if name == "" {
		h.renderToast(w, "error", "Name is required")
		return
	}

	if color == "" {
		color = "#3B82F6"
	}
	if !hexColorRegexp.MatchString(color) {
		h.renderToast(w, "error", "Invalid color format")
		return
	}

	if avatarEmoji == "" {
		avatarEmoji = "\xf0\x9f\x98\x80"
	}

	exists, err := h.store.NameExists(name, 0)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if exists {
		h.renderToast(w, "error", "A family member with that name already exists")
		return
	}

	if _, err := h.store.Create(name, color, avatarEmoji); err != nil {
		http.Error(w, "failed to create family member", http.StatusInternalServerError)
		return
	}

	members, err := h.store.List()
	if err != nil {
		http.Error(w, "failed to load family members", http.StatusInternalServerError)
		return
	}

	h.renderPartial(w, "member-list", map[string]any{"Members": members})
}

func (h *TemplateHandler) FamilyMemberUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	color := r.FormValue("color")
	avatarEmoji := r.FormValue("avatar_emoji")

	if name == "" {
		h.renderToast(w, "error", "Name is required")
		return
	}

	if !hexColorRegexp.MatchString(color) {
		h.renderToast(w, "error", "Invalid color format")
		return
	}

	exists, err := h.store.NameExists(name, id)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if exists {
		h.renderToast(w, "error", "A family member with that name already exists")
		return
	}

	if _, err := h.store.Update(id, name, color, avatarEmoji); err != nil {
		http.Error(w, "failed to update family member", http.StatusInternalServerError)
		return
	}

	members, err := h.store.List()
	if err != nil {
		http.Error(w, "failed to load family members", http.StatusInternalServerError)
		return
	}

	h.renderPartial(w, "member-list", map[string]any{"Members": members})
}

func (h *TemplateHandler) FamilyMemberDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := h.store.Delete(id); err != nil {
		http.Error(w, "failed to delete family member", http.StatusInternalServerError)
		return
	}

	members, err := h.store.List()
	if err != nil {
		http.Error(w, "failed to load family members", http.StatusInternalServerError)
		return
	}

	h.renderPartial(w, "member-list", map[string]any{"Members": members})
}

func (h *TemplateHandler) PINSetupForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	member, err := h.store.GetByID(id)
	if err != nil || member == nil {
		http.Error(w, "family member not found", http.StatusNotFound)
		return
	}

	if member.HasPIN {
		h.renderPartial(w, "pin-manage", member)
	} else {
		h.renderPartial(w, "pin-setup", member)
	}
}

func (h *TemplateHandler) PINGate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	member, err := h.store.GetByID(id)
	if err != nil || member == nil {
		http.Error(w, "family member not found", http.StatusNotFound)
		return
	}

	nextAction := r.URL.Query().Get("next")
	if nextAction != "change" && nextAction != "clear" {
		nextAction = "change"
	}

	data := struct {
		ID         int64
		Name       string
		NextAction string
	}{member.ID, member.Name, nextAction}

	h.renderPartial(w, "pin-verify", data)
}

func (h *TemplateHandler) PINVerifyThenAct(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	pin := r.FormValue("pin")
	nextAction := r.FormValue("next_action")

	hash, err := h.store.GetPINHash(id)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(pin)); err != nil {
		h.renderToast(w, "error", "Incorrect PIN")

		member, _ := h.store.GetByID(id)
		data := struct {
			ID         int64
			Name       string
			NextAction string
		}{member.ID, member.Name, nextAction}
		h.renderPartial(w, "pin-verify", data)
		return
	}

	member, _ := h.store.GetByID(id)

	if nextAction == "clear" {
		if err := h.store.ClearPIN(id); err != nil {
			http.Error(w, "failed to clear PIN", http.StatusInternalServerError)
			return
		}
		h.renderToast(w, "success", "PIN removed")
		members, _ := h.store.List()
		h.renderPartial(w, "member-list", map[string]any{"Members": members})
		return
	}

	// nextAction == "change": show the set-PIN form
	h.renderPartial(w, "pin-setup", member)
}

func (h *TemplateHandler) PINSetup(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	pin := r.FormValue("pin")

	if len(pin) != 4 || !isDigits(pin) {
		h.renderToast(w, "error", "PIN must be exactly 4 digits")
		member, _ := h.store.GetByID(id)
		h.renderPartial(w, "pin-setup", member)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "failed to hash PIN", http.StatusInternalServerError)
		return
	}

	if err := h.store.SetPIN(id, string(hash)); err != nil {
		http.Error(w, "failed to set PIN", http.StatusInternalServerError)
		return
	}

	h.renderToast(w, "success", "PIN set successfully")

	members, err := h.store.List()
	if err != nil {
		http.Error(w, "failed to load family members", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "member-list", map[string]any{"Members": members})
}

// --- Calendar helper types and methods ---

type dayViewEvent struct {
	ID          int64
	Title       string
	Color       string
	MemberEmoji string
	TimeRange   string
	TopPx       int
	HeightPx    int
}

type weekDayEvent struct {
	Title string
	Color string
}

type weekDay struct {
	DayName   string
	DayNum    int
	DateStr   string
	IsToday   bool
	Events    []weekDayEvent
	MoreCount int
}

type hourSlot struct {
	Hour  int
	Label string
	TopPx int
}

func (h *TemplateHandler) parseCalendarDate(r *http.Request) time.Time {
	dateStr := r.URL.Query().Get("date")
	if dateStr != "" {
		if t, err := time.Parse("2006-01-02", dateStr); err == nil {
			return t
		}
	}
	return time.Now()
}

func (h *TemplateHandler) buildCalendarViewContent(view string, date time.Time) (template.HTML, error) {
	var templateName string
	var data any
	var err error

	if view == "week" {
		templateName = "calendar-week-view"
		data, err = h.buildWeekViewData(date)
	} else {
		templateName = "calendar-day-view"
		data, err = h.buildDayViewData(date)
	}
	if err != nil {
		return "", err
	}

	return h.renderSection(templateName, data)
}

func (h *TemplateHandler) buildDayViewData(date time.Time) (map[string]any, error) {
	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	dayEnd := dayStart.Add(24 * time.Hour)

	events, err := h.eventStore.ListByDateRange(dayStart, dayEnd)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}

	members, err := h.store.List()
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	memberMap := make(map[int64]model.FamilyMember)
	for _, m := range members {
		memberMap[m.ID] = m
	}

	var allDayEvents []dayViewEvent
	var timedEvents []dayViewEvent

	for _, e := range events {
		color := "#6B7280" // default gray
		emoji := ""
		if e.FamilyMemberID != nil {
			if m, ok := memberMap[*e.FamilyMemberID]; ok {
				color = m.Color
				emoji = m.AvatarEmoji
			}
		}

		if e.AllDay {
			allDayEvents = append(allDayEvents, dayViewEvent{
				ID:          e.ID,
				Title:       e.Title,
				Color:       color,
				MemberEmoji: emoji,
			})
			continue
		}

		// Calculate position relative to 6am baseline
		baseHour := 6
		startMinutes := (e.StartTime.Hour()-baseHour)*60 + e.StartTime.Minute()
		endMinutes := (e.EndTime.Hour()-baseHour)*60 + e.EndTime.Minute()

		// Clamp to visible range (6am-11pm = 0 to 1020px)
		if startMinutes < 0 {
			startMinutes = 0
		}
		if endMinutes > 17*60 {
			endMinutes = 17 * 60
		}
		if endMinutes <= startMinutes {
			continue
		}

		topPx := startMinutes // 1 minute = 1px (60px per hour)
		heightPx := endMinutes - startMinutes
		if heightPx < 24 {
			heightPx = 24
		}

		timeRange := e.StartTime.Format("3:04 PM") + " - " + e.EndTime.Format("3:04 PM")

		timedEvents = append(timedEvents, dayViewEvent{
			ID:          e.ID,
			Title:       e.Title,
			Color:       color,
			MemberEmoji: emoji,
			TimeRange:   timeRange,
			TopPx:       topPx,
			HeightPx:    heightPx,
		})
	}

	// Build hour slots (6am to 10pm)
	hours := make([]hourSlot, 17)
	for i := 0; i < 17; i++ {
		h := i + 6
		label := fmt.Sprintf("%d AM", h)
		if h == 12 {
			label = "12 PM"
		} else if h > 12 {
			label = fmt.Sprintf("%d PM", h-12)
		}
		hours[i] = hourSlot{
			Hour:  h,
			Label: label,
			TopPx: i * 60,
		}
	}

	today := time.Now()
	isToday := date.Year() == today.Year() && date.YearDay() == today.YearDay()

	return map[string]any{
		"DateStr":     dayStart.Format("2006-01-02"),
		"DateLabel":   dayStart.Format("Monday, January 2, 2006"),
		"PrevDate":    dayStart.AddDate(0, 0, -1).Format("2006-01-02"),
		"NextDate":    dayStart.AddDate(0, 0, 1).Format("2006-01-02"),
		"IsToday":     isToday,
		"AllDayEvents": allDayEvents,
		"TimedEvents": timedEvents,
		"Hours":       hours,
	}, nil
}

func (h *TemplateHandler) buildWeekViewData(date time.Time) (map[string]any, error) {
	// Find Monday of the week (ISO 8601)
	weekday := date.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	monday := date.AddDate(0, 0, -int(weekday-time.Monday))
	monday = time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, time.UTC)
	sunday := monday.AddDate(0, 0, 7)

	events, err := h.eventStore.ListByDateRange(monday, sunday)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}

	members, err := h.store.List()
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	memberMap := make(map[int64]model.FamilyMember)
	for _, m := range members {
		memberMap[m.ID] = m
	}

	today := time.Now()
	days := make([]weekDay, 7)

	for i := 0; i < 7; i++ {
		d := monday.AddDate(0, 0, i)
		dayStart := d
		dayEnd := d.AddDate(0, 0, 1)

		isToday := d.Year() == today.Year() && d.YearDay() == today.YearDay()

		var dayEvents []weekDayEvent
		for _, e := range events {
			// Check overlap with this day
			if e.StartTime.Before(dayEnd) && e.EndTime.After(dayStart) {
				color := "#6B7280"
				if e.FamilyMemberID != nil {
					if m, ok := memberMap[*e.FamilyMemberID]; ok {
						color = m.Color
					}
				}
				dayEvents = append(dayEvents, weekDayEvent{
					Title: e.Title,
					Color: color,
				})
			}
		}

		moreCount := 0
		displayEvents := dayEvents
		if len(dayEvents) > 3 {
			displayEvents = dayEvents[:3]
			moreCount = len(dayEvents) - 3
		}

		days[i] = weekDay{
			DayName:   d.Format("Mon"),
			DayNum:    d.Day(),
			DateStr:   d.Format("2006-01-02"),
			IsToday:   isToday,
			Events:    displayEvents,
			MoreCount: moreCount,
		}
	}

	weekLabel := fmt.Sprintf("%s - %s", monday.Format("Jan 2"), monday.AddDate(0, 0, 6).Format("Jan 2, 2006"))

	return map[string]any{
		"Days":     days,
		"WeekLabel": weekLabel,
		"PrevWeek": monday.AddDate(0, 0, -7).Format("2006-01-02"),
		"NextWeek": monday.AddDate(0, 0, 7).Format("2006-01-02"),
		"TodayStr": today.Format("2006-01-02"),
	}, nil
}

func (h *TemplateHandler) buildEventDetailData(event *model.CalendarEvent) map[string]any {
	data := map[string]any{
		"ID":          event.ID,
		"Title":       event.Title,
		"Description": event.Description,
		"Location":    event.Location,
		"AllDay":      event.AllDay,
	}

	if event.AllDay {
		start := event.StartTime.Format("Mon, Jan 2")
		end := event.EndTime.Add(-24 * time.Hour).Format("Mon, Jan 2, 2006")
		if start == end {
			data["DateRange"] = start
		} else {
			data["DateRange"] = start + " - " + end
		}
	} else {
		data["TimeRange"] = event.StartTime.Format("3:04 PM") + " - " + event.EndTime.Format("3:04 PM")
		data["DateLabel"] = event.StartTime.Format("Monday, January 2, 2006")
	}

	if event.FamilyMemberID != nil {
		member, err := h.store.GetByID(*event.FamilyMemberID)
		if err == nil && member != nil {
			data["MemberName"] = member.Name
			data["MemberColor"] = member.Color
			data["MemberEmoji"] = member.AvatarEmoji
		}
	}

	return data
}

func (h *TemplateHandler) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("template error: %v", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (h *TemplateHandler) renderPartial(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("template error rendering %q: %v", name, err)
		fmt.Fprintf(w, `<div class="alert alert-error">Template error</div>`)
	}
}

func (h *TemplateHandler) renderToast(w http.ResponseWriter, level, message string) {
	tmplName := "toast-success"
	if level == "error" {
		tmplName = "toast-error"
	}
	h.templates.ExecuteTemplate(w, tmplName, map[string]string{"Message": message})
}
