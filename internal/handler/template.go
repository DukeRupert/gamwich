package handler

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dukerupert/gamwich/internal/auth"
	"github.com/dukerupert/gamwich/internal/backup"
	"github.com/dukerupert/gamwich/internal/chore"

	"github.com/dukerupert/gamwich/internal/grocery"
	"github.com/dukerupert/gamwich/internal/license"
	"github.com/dukerupert/gamwich/internal/model"
	"github.com/dukerupert/gamwich/internal/push"
	"github.com/dukerupert/gamwich/internal/recurrence"
	"github.com/dukerupert/gamwich/internal/store"
	"github.com/dukerupert/gamwich/internal/tunnel"
	"github.com/dukerupert/gamwich/internal/weather"
	"github.com/dukerupert/gamwich/internal/websocket"
	"golang.org/x/crypto/bcrypt"
)

const activeUserCookie = "gamwich_active_user"

type TemplateHandler struct {
	store          *store.FamilyMemberStore
	eventStore     *store.EventStore
	choreStore     *store.ChoreStore
	groceryStore   *store.GroceryStore
	noteStore      *store.NoteStore
	rewardStore    *store.RewardStore
	settingsStore  *store.SettingsStore
	weatherSvc     *weather.Service
	hub            *websocket.Hub
	licenseClient  *license.Client
	tunnelManager  *tunnel.Manager
	backupManager  *backup.Manager
	backupStore    *store.BackupStore
	pushStore      *store.PushStore
	pushService    *push.Service
	pushScheduler  *push.Scheduler
	templates      *template.Template
	logger         *slog.Logger
}

func NewTemplateHandler(s *store.FamilyMemberStore, es *store.EventStore, cs *store.ChoreStore, gs *store.GroceryStore, ns *store.NoteStore, rs *store.RewardStore, ss *store.SettingsStore, w *weather.Service, hub *websocket.Hub, lc *license.Client, tm *tunnel.Manager, bm *backup.Manager, bs *store.BackupStore, ps *store.PushStore, pushSvc *push.Service, pushSched *push.Scheduler, logger *slog.Logger) *TemplateHandler {
	funcMap := template.FuncMap{
		"add":         func(a, b int) int { return a + b },
		"formatBytes": formatBytes,
		"seq": func(start, end int) []int {
			s := make([]int, 0, end-start+1)
			for i := start; i <= end; i++ {
				s = append(s, i)
			}
			return s
		},
	}
	tmpl := template.Must(template.New("").Funcs(funcMap).ParseGlob("web/templates/*.html"))
	return &TemplateHandler{
		store:         s,
		eventStore:    es,
		choreStore:    cs,
		groceryStore:  gs,
		noteStore:     ns,
		rewardStore:   rs,
		settingsStore: ss,
		weatherSvc:    w,
		hub:           hub,
		licenseClient: lc,
		tunnelManager: tm,
		backupManager: bm,
		backupStore:   bs,
		pushStore:     ps,
		pushService:   pushSvc,
		pushScheduler: pushSched,
		templates:     tmpl,
		logger:        logger,
	}
}

func formatBytes(b int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func (h *TemplateHandler) broadcast(msg websocket.Message) {
	if h.hub != nil {
		h.hub.Broadcast(msg)
	}
}

// buildDashboardData assembles the common data map for layout rendering.
func (h *TemplateHandler) buildDashboardData(r *http.Request, section string) (map[string]any, error) {
	members, err := h.store.List()
	if err != nil {
		return nil, fmt.Errorf("failed to load members: %w", err)
	}

	activeUserID := h.activeUserFromCookie(r)

	kioskSettings, _ := h.settingsStore.GetKioskSettings()
	themeSettings, _ := h.settingsStore.GetThemeSettings()

	data := map[string]any{
		"Title":          "Gamwich",
		"Members":        members,
		"ActiveUserID":   activeUserID,
		"ActiveSection":  section,
		"Weather":        h.weatherSvc.GetWeather(),
		"KioskSettings":  kioskSettings,
		"ThemeSettings":  themeSettings,
		"IsFreeTier":     h.licenseClient.IsFreeTier(),
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

	choreSummary, _ := h.buildChoreSummaryData()
	data["ChoreSummary"] = choreSummary

	grocerySummary, _ := h.buildGrocerySummaryData()
	data["GrocerySummary"] = grocerySummary

	noteSummary, _ := h.buildNoteSummaryData()
	data["NoteSummary"] = noteSummary

	leaderboard, _ := h.buildLeaderboardData()
	data["Leaderboard"] = leaderboard

	content, err := h.renderSection("dashboard-content", data)
	if err != nil {
		h.logger.Error("render dashboard content", "error", err)
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
			h.logger.Error("render section content", "section", section, "error", err)
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

	choreSummary, _ := h.buildChoreSummaryData()
	data["ChoreSummary"] = choreSummary

	grocerySummary, _ := h.buildGrocerySummaryData()
	data["GrocerySummary"] = grocerySummary

	noteSummary, _ := h.buildNoteSummaryData()
	data["NoteSummary"] = noteSummary

	leaderboard, _ := h.buildLeaderboardData()
	data["Leaderboard"] = leaderboard

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
		h.logger.Error("build calendar view", "error", err)
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
		h.logger.Error("render calendar content", "error", err)
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
		h.logger.Error("build calendar view", "error", err)
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
		h.logger.Error("build day view", "error", err)
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
		h.logger.Error("build week view", "error", err)
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

	// Check query params for recurring event context
	isRecurring := r.URL.Query().Get("recurring") == "1"
	parentID, _ := strconv.ParseInt(r.URL.Query().Get("parent_id"), 10, 64)
	occurrenceDate := r.URL.Query().Get("occurrence_date")

	// If this event itself is a recurring parent, mark it as such
	if event.RecurrenceRule != "" {
		isRecurring = true
		parentID = event.ID
	}

	data := h.buildEventDetailData(event, isRecurring, parentID, occurrenceDate)
	h.renderPartial(w, "calendar-event-detail", data)
}

// CalendarEventNewForm renders the event creation form in the modal.
func (h *TemplateHandler) CalendarEventNewForm(w http.ResponseWriter, r *http.Request) {
	date := h.parseCalendarDate(r)
	hourStr := r.URL.Query().Get("hour")
	hour := 9 // default
	if h, err := strconv.Atoi(hourStr); err == nil && h >= 0 && h <= 23 {
		hour = h
	}

	members, _ := h.store.List()

	data := map[string]any{
		"DateYear":    date.Year(),
		"DateMonth":   int(date.Month()),
		"DateDay":     date.Day(),
		"StartHour":   hour,
		"StartMinute": 0,
		"EndHour":     hour + 1,
		"EndMinute":   0,
		"Members":     members,
	}
	h.renderPartial(w, "event-form", data)
}

// CalendarEventCreate handles POST form submission to create an event.
func (h *TemplateHandler) CalendarEventCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		h.renderToast(w, "error", "Title is required")
		return
	}

	dateStr := r.FormValue("date")
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		h.renderToast(w, "error", "Invalid date")
		return
	}

	allDay := r.FormValue("all_day") == "1"
	description := r.FormValue("description")
	location := r.FormValue("location")

	var startTime, endTime time.Time
	if allDay {
		startTime = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
		endTime = startTime.Add(24 * time.Hour)
	} else {
		startHour, _ := strconv.Atoi(r.FormValue("start_hour"))
		startMinute, _ := strconv.Atoi(r.FormValue("start_minute"))
		endHour, _ := strconv.Atoi(r.FormValue("end_hour"))
		endMinute, _ := strconv.Atoi(r.FormValue("end_minute"))

		startTime = time.Date(date.Year(), date.Month(), date.Day(), startHour, startMinute, 0, 0, time.UTC)
		endTime = time.Date(date.Year(), date.Month(), date.Day(), endHour, endMinute, 0, 0, time.UTC)

		if !startTime.Before(endTime) {
			h.renderToast(w, "error", "Start time must be before end time")
			return
		}
	}

	var familyMemberID *int64
	if memberStr := r.FormValue("family_member_id"); memberStr != "" && memberStr != "0" {
		id, err := strconv.ParseInt(memberStr, 10, 64)
		if err == nil {
			familyMemberID = &id
		}
	}

	recurrenceRule := r.FormValue("recurrence_rule")

	event, err := h.eventStore.CreateWithRecurrence(title, description, startTime, endTime, allDay, familyMemberID, location, recurrenceRule)
	if err != nil {
		h.logger.Error("create event", "error", err)
		http.Error(w, "failed to create event", http.StatusInternalServerError)
		return
	}

	if rmStr := r.FormValue("reminder_minutes"); rmStr != "" {
		if rm, err := strconv.Atoi(rmStr); err == nil {
			if err := h.eventStore.SetReminderMinutes(event.ID, &rm); err != nil {
				h.logger.Error("set reminder", "error", err)
			}
		}
	}

	h.broadcast(websocket.NewMessage("calendar_event", "created", event.ID, nil))

	w.Header().Set("HX-Trigger", "closeEventModal")

	// Re-render the current day view
	data, err := h.buildDayViewData(date)
	if err != nil {
		http.Error(w, "failed to load events", http.StatusInternalServerError)
		return
	}
	h.renderToast(w, "success", "Event created")
	h.renderPartial(w, "calendar-day-view", data)
}

// CalendarEventEditForm renders the edit form for an event.
func (h *TemplateHandler) CalendarEventEditForm(w http.ResponseWriter, r *http.Request) {
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

	members, _ := h.store.List()

	var familyMemberID int64
	if event.FamilyMemberID != nil {
		familyMemberID = *event.FamilyMemberID
	}

	// For recurring events, accept mode/occurrence context from query params
	mode := r.URL.Query().Get("mode")
	parentID, _ := strconv.ParseInt(r.URL.Query().Get("parent_id"), 10, 64)
	occurrenceDate := r.URL.Query().Get("occurrence_date")

	var reminderMinutes string
	if event.ReminderMinutes != nil {
		reminderMinutes = strconv.Itoa(*event.ReminderMinutes)
	}

	data := map[string]any{
		"ID":              event.ID,
		"Title":           event.Title,
		"Description":     event.Description,
		"Location":        event.Location,
		"AllDay":          event.AllDay,
		"DateYear":        event.StartTime.Year(),
		"DateMonth":       int(event.StartTime.Month()),
		"DateDay":         event.StartTime.Day(),
		"StartHour":       event.StartTime.Hour(),
		"StartMinute":     event.StartTime.Minute(),
		"EndHour":         event.EndTime.Hour(),
		"EndMinute":       event.EndTime.Minute(),
		"FamilyMemberID":  familyMemberID,
		"Members":         members,
		"RecurrenceRule":  event.RecurrenceRule,
		"ReminderMinutes": reminderMinutes,
		"Mode":            mode,
		"ParentID":        parentID,
		"OccurrenceDate":  occurrenceDate,
		"IsRecurring":     event.RecurrenceRule != "" || parentID > 0,
	}

	h.renderPartial(w, "event-edit-form", data)
}

// CalendarEventUpdate handles PUT form submission to update an event.
func (h *TemplateHandler) CalendarEventUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		h.renderToast(w, "error", "Title is required")
		return
	}

	dateStr := r.FormValue("date")
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		h.renderToast(w, "error", "Invalid date")
		return
	}

	allDay := r.FormValue("all_day") == "1"
	description := r.FormValue("description")
	location := r.FormValue("location")

	var startTime, endTime time.Time
	if allDay {
		startTime = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
		endTime = startTime.Add(24 * time.Hour)
	} else {
		startHour, _ := strconv.Atoi(r.FormValue("start_hour"))
		startMinute, _ := strconv.Atoi(r.FormValue("start_minute"))
		endHour, _ := strconv.Atoi(r.FormValue("end_hour"))
		endMinute, _ := strconv.Atoi(r.FormValue("end_minute"))

		startTime = time.Date(date.Year(), date.Month(), date.Day(), startHour, startMinute, 0, 0, time.UTC)
		endTime = time.Date(date.Year(), date.Month(), date.Day(), endHour, endMinute, 0, 0, time.UTC)

		if !startTime.Before(endTime) {
			h.renderToast(w, "error", "Start time must be before end time")
			return
		}
	}

	var familyMemberID *int64
	if memberStr := r.FormValue("family_member_id"); memberStr != "" && memberStr != "0" {
		mid, err := strconv.ParseInt(memberStr, 10, 64)
		if err == nil {
			familyMemberID = &mid
		}
	}

	recurrenceRule := r.FormValue("recurrence_rule")
	mode := r.FormValue("mode")
	parentIDStr := r.FormValue("parent_id")
	occurrenceDateStr := r.FormValue("occurrence_date")

	switch mode {
	case "single":
		// Create an exception for this occurrence
		parentID, _ := strconv.ParseInt(parentIDStr, 10, 64)
		if parentID == 0 {
			parentID = id
		}
		parent, err := h.eventStore.GetByID(parentID)
		if err != nil || parent == nil {
			http.Error(w, "parent event not found", http.StatusNotFound)
			return
		}
		origStart := parent.StartTime
		if occurrenceDateStr != "" {
			if od, err := time.Parse("2006-01-02", occurrenceDateStr); err == nil {
				origStart = time.Date(od.Year(), od.Month(), od.Day(), parent.StartTime.Hour(), parent.StartTime.Minute(), 0, 0, time.UTC)
			}
		}
		if _, err := h.eventStore.CreateException(parentID, origStart, title, description, startTime, endTime, allDay, familyMemberID, location, false); err != nil {
			h.logger.Error("create exception", "error", err)
			http.Error(w, "failed to create exception", http.StatusInternalServerError)
			return
		}

	case "all":
		// Update the parent event directly and delete all exceptions
		parentID, _ := strconv.ParseInt(parentIDStr, 10, 64)
		if parentID == 0 {
			parentID = id
		}
		if err := h.eventStore.DeleteExceptions(parentID); err != nil {
			h.logger.Error("delete exceptions", "error", err)
		}
		if _, err := h.eventStore.UpdateWithRecurrence(parentID, title, description, startTime, endTime, allDay, familyMemberID, location, recurrenceRule); err != nil {
			h.logger.Error("update event", "error", err)
			http.Error(w, "failed to update event", http.StatusInternalServerError)
			return
		}

	default:
		// Non-recurring or simple update
		if _, err := h.eventStore.UpdateWithRecurrence(id, title, description, startTime, endTime, allDay, familyMemberID, location, recurrenceRule); err != nil {
			h.logger.Error("update event", "error", err)
			http.Error(w, "failed to update event", http.StatusInternalServerError)
			return
		}
	}

	// Update reminder for the target event
	if rmStr := r.FormValue("reminder_minutes"); rmStr != "" {
		if rm, err := strconv.Atoi(rmStr); err == nil {
			if err := h.eventStore.SetReminderMinutes(id, &rm); err != nil {
				h.logger.Error("set reminder", "error", err)
			}
		}
	} else {
		// Clear reminder if empty
		if err := h.eventStore.SetReminderMinutes(id, nil); err != nil {
			h.logger.Error("clear reminder", "error", err)
		}
	}

	h.broadcast(websocket.NewMessage("calendar_event", "updated", id, nil))

	w.Header().Set("HX-Trigger", "closeEventModal")

	data, err := h.buildDayViewData(date)
	if err != nil {
		http.Error(w, "failed to load events", http.StatusInternalServerError)
		return
	}
	h.renderToast(w, "success", "Event updated")
	h.renderPartial(w, "calendar-day-view", data)
}

// CalendarEventDeleteForm handles DELETE from the event detail modal.
func (h *TemplateHandler) CalendarEventDeleteForm(w http.ResponseWriter, r *http.Request) {
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

	date := event.StartTime

	mode := r.URL.Query().Get("mode")
	parentIDStr := r.URL.Query().Get("parent_id")
	occurrenceDateStr := r.URL.Query().Get("occurrence_date")

	switch mode {
	case "single":
		// Create a cancelled exception
		parentID, _ := strconv.ParseInt(parentIDStr, 10, 64)
		if parentID == 0 {
			parentID = id
		}
		parent, err := h.eventStore.GetByID(parentID)
		if err != nil || parent == nil {
			http.Error(w, "parent event not found", http.StatusNotFound)
			return
		}
		origStart := parent.StartTime
		if occurrenceDateStr != "" {
			if od, err := time.Parse("2006-01-02", occurrenceDateStr); err == nil {
				origStart = time.Date(od.Year(), od.Month(), od.Day(), parent.StartTime.Hour(), parent.StartTime.Minute(), 0, 0, time.UTC)
				date = od
			}
		}
		if _, err := h.eventStore.CreateException(parentID, origStart, parent.Title, parent.Description, origStart, origStart.Add(parent.EndTime.Sub(parent.StartTime)), parent.AllDay, parent.FamilyMemberID, parent.Location, true); err != nil {
			h.logger.Error("create cancelled exception", "error", err)
			http.Error(w, "failed to delete occurrence", http.StatusInternalServerError)
			return
		}

	case "all":
		// Delete the parent (CASCADE removes exceptions)
		parentID, _ := strconv.ParseInt(parentIDStr, 10, 64)
		if parentID == 0 {
			parentID = id
		}
		if err := h.eventStore.Delete(parentID); err != nil {
			http.Error(w, "failed to delete event series", http.StatusInternalServerError)
			return
		}

	default:
		// Non-recurring delete
		if err := h.eventStore.Delete(id); err != nil {
			http.Error(w, "failed to delete event", http.StatusInternalServerError)
			return
		}
	}

	h.broadcast(websocket.NewMessage("calendar_event", "deleted", id, nil))

	w.Header().Set("HX-Trigger", "closeEventModal")

	data, err := h.buildDayViewData(date)
	if err != nil {
		http.Error(w, "failed to load events", http.StatusInternalServerError)
		return
	}
	h.renderToast(w, "success", "Event deleted")
	h.renderPartial(w, "calendar-day-view", data)
}

// RecurrenceEditChoice renders the edit mode choice dialog for recurring events.
func (h *TemplateHandler) RecurrenceEditChoice(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	parentID, _ := strconv.ParseInt(r.URL.Query().Get("parent_id"), 10, 64)
	occurrenceDate := r.URL.Query().Get("occurrence_date")

	data := map[string]any{
		"ID":             id,
		"ParentID":       parentID,
		"OccurrenceDate": occurrenceDate,
	}
	h.renderPartial(w, "recurrence-edit-choice", data)
}

// RecurrenceDeleteChoice renders the delete mode choice dialog for recurring events.
func (h *TemplateHandler) RecurrenceDeleteChoice(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	parentID, _ := strconv.ParseInt(r.URL.Query().Get("parent_id"), 10, 64)
	occurrenceDate := r.URL.Query().Get("occurrence_date")

	data := map[string]any{
		"ID":             id,
		"ParentID":       parentID,
		"OccurrenceDate": occurrenceDate,
	}
	h.renderPartial(w, "recurrence-delete-choice", data)
}

// ChoresPage renders the full chores page inside the dashboard layout.
func (h *TemplateHandler) ChoresPage(w http.ResponseWriter, r *http.Request) {
	data, err := h.buildDashboardData(r, "chores")
	if err != nil {
		http.Error(w, "failed to load data", http.StatusInternalServerError)
		return
	}

	choreData, err := h.buildChoreListData("all", 0)
	if err != nil {
		h.logger.Error("build chore list", "error", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}

	content, err := h.renderSection("chores-content", choreData)
	if err != nil {
		h.logger.Error("render chores content", "error", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	data["Content"] = content

	h.render(w, "layout.html", data)
}

// ChoresPartial renders the chores section for HTMX swap.
func (h *TemplateHandler) ChoresPartial(w http.ResponseWriter, r *http.Request) {
	choreData, err := h.buildChoreListData("all", 0)
	if err != nil {
		h.logger.Error("build chore list", "error", err)
		http.Error(w, "failed to load chores", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "chores-content", choreData)
}

// ChoreListByPerson renders the by-person chore list partial.
func (h *TemplateHandler) ChoreListByPerson(w http.ResponseWriter, r *http.Request) {
	choreData, err := h.buildChoreListData("by-person", 0)
	if err != nil {
		http.Error(w, "failed to load chores", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "chore-list-by-person", choreData)
}

// ChoreListByArea renders the by-area chore list partial.
func (h *TemplateHandler) ChoreListByArea(w http.ResponseWriter, r *http.Request) {
	choreData, err := h.buildChoreListData("by-area", 0)
	if err != nil {
		http.Error(w, "failed to load chores", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "chore-list-by-area", choreData)
}

// ChoreListAll renders the all chores list partial.
func (h *TemplateHandler) ChoreListAll(w http.ResponseWriter, r *http.Request) {
	choreData, err := h.buildChoreListData("all", 0)
	if err != nil {
		http.Error(w, "failed to load chores", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "chore-list-all", choreData)
}

// ChoreNewForm renders the chore creation form.
func (h *TemplateHandler) ChoreNewForm(w http.ResponseWriter, r *http.Request) {
	members, _ := h.store.List()
	areas, _ := h.choreStore.ListAreas()

	h.renderPartial(w, "chore-form", map[string]any{
		"Members": members,
		"Areas":   areas,
	})
}

// ChoreEditForm renders the chore edit form.
func (h *TemplateHandler) ChoreEditForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	c, err := h.choreStore.GetByID(id)
	if err != nil || c == nil {
		http.Error(w, "chore not found", http.StatusNotFound)
		return
	}

	members, _ := h.store.List()
	areas, _ := h.choreStore.ListAreas()

	var areaID, assignedTo int64
	if c.AreaID != nil {
		areaID = *c.AreaID
	}
	if c.AssignedTo != nil {
		assignedTo = *c.AssignedTo
	}

	h.renderPartial(w, "chore-edit-form", map[string]any{
		"ID":             c.ID,
		"Title":          c.Title,
		"Description":    c.Description,
		"AreaID":         areaID,
		"Points":         c.Points,
		"RecurrenceRule": c.RecurrenceRule,
		"AssignedTo":     assignedTo,
		"Members":        members,
		"Areas":          areas,
	})
}

// ChoreCreate handles POST form submission to create a chore.
func (h *TemplateHandler) ChoreCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		h.renderToast(w, "error", "Title is required")
		return
	}

	description := r.FormValue("description")
	points, _ := strconv.Atoi(r.FormValue("points"))
	recurrenceRule := r.FormValue("recurrence_rule")

	var areaID *int64
	if aStr := r.FormValue("area_id"); aStr != "" && aStr != "0" {
		id, err := strconv.ParseInt(aStr, 10, 64)
		if err == nil {
			areaID = &id
		}
	}

	var assignedTo *int64
	if mStr := r.FormValue("assigned_to"); mStr != "" && mStr != "0" {
		id, err := strconv.ParseInt(mStr, 10, 64)
		if err == nil {
			assignedTo = &id
		}
	}

	newChore, err := h.choreStore.Create(title, description, areaID, points, recurrenceRule, assignedTo)
	if err != nil {
		h.logger.Error("create chore", "error", err)
		http.Error(w, "failed to create chore", http.StatusInternalServerError)
		return
	}

	h.broadcast(websocket.NewMessage("chore", "created", newChore.ID, nil))

	w.Header().Set("HX-Trigger", "closeChoreModal")
	h.renderToast(w, "success", "Chore created")

	choreData, err := h.buildChoreListData("all", 0)
	if err != nil {
		http.Error(w, "failed to load chores", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "chore-list-all", choreData)
}

// ChoreUpdate handles PUT form submission to update a chore.
func (h *TemplateHandler) ChoreUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		h.renderToast(w, "error", "Title is required")
		return
	}

	description := r.FormValue("description")
	points, _ := strconv.Atoi(r.FormValue("points"))
	recurrenceRule := r.FormValue("recurrence_rule")

	var areaID *int64
	if aStr := r.FormValue("area_id"); aStr != "" && aStr != "0" {
		aid, err := strconv.ParseInt(aStr, 10, 64)
		if err == nil {
			areaID = &aid
		}
	}

	var assignedTo *int64
	if mStr := r.FormValue("assigned_to"); mStr != "" && mStr != "0" {
		mid, err := strconv.ParseInt(mStr, 10, 64)
		if err == nil {
			assignedTo = &mid
		}
	}

	if _, err := h.choreStore.Update(id, title, description, areaID, points, recurrenceRule, assignedTo); err != nil {
		h.logger.Error("update chore", "error", err)
		http.Error(w, "failed to update chore", http.StatusInternalServerError)
		return
	}

	h.broadcast(websocket.NewMessage("chore", "updated", id, nil))

	w.Header().Set("HX-Trigger", "closeChoreModal")
	h.renderToast(w, "success", "Chore updated")

	choreData, err := h.buildChoreListData("all", 0)
	if err != nil {
		http.Error(w, "failed to load chores", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "chore-list-all", choreData)
}

// ChoreDelete handles DELETE for a chore.
func (h *TemplateHandler) ChoreDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := h.choreStore.Delete(id); err != nil {
		http.Error(w, "failed to delete chore", http.StatusInternalServerError)
		return
	}

	h.broadcast(websocket.NewMessage("chore", "deleted", id, nil))

	w.Header().Set("HX-Trigger", "closeChoreModal")
	h.renderToast(w, "success", "Chore deleted")

	choreData, err := h.buildChoreListData("all", 0)
	if err != nil {
		http.Error(w, "failed to load chores", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "chore-list-all", choreData)
}

// ChoreComplete handles POST to complete a chore via HTMX.
func (h *TemplateHandler) ChoreComplete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	activeUserID := h.activeUserFromCookie(r)
	var completedBy *int64
	if activeUserID > 0 {
		completedBy = &activeUserID
	}

	// Look up chore to get points
	choreObj, err := h.choreStore.GetByID(id)
	if err != nil || choreObj == nil {
		http.Error(w, "chore not found", http.StatusNotFound)
		return
	}

	if _, err := h.choreStore.CreateCompletion(id, completedBy, choreObj.Points); err != nil {
		h.logger.Error("complete chore", "error", err)
		http.Error(w, "failed to complete chore", http.StatusInternalServerError)
		return
	}

	h.broadcast(websocket.NewMessage("chore", "completed", id, nil))

	toastMsg := "Chore completed!"
	if choreObj.Points > 0 {
		toastMsg = fmt.Sprintf("Chore completed! +%d points", choreObj.Points)
	}
	h.renderToast(w, "success", toastMsg)

	choreData, err := h.buildChoreListData("all", 0)
	if err != nil {
		http.Error(w, "failed to load chores", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "chore-list-all", choreData)
}

// ChoreUndoComplete handles DELETE to undo a chore completion.
func (h *TemplateHandler) ChoreUndoComplete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	// Undo by removing the last completion for this chore
	last, err := h.choreStore.LastCompletionForChore(id)
	if err != nil {
		http.Error(w, "failed to get completion", http.StatusInternalServerError)
		return
	}
	if last != nil {
		if err := h.choreStore.DeleteCompletion(last.ID); err != nil {
			http.Error(w, "failed to undo completion", http.StatusInternalServerError)
			return
		}
	}

	h.broadcast(websocket.NewMessage("chore", "completion_undone", id, nil))

	h.renderToast(w, "success", "Completion undone")

	choreData, err := h.buildChoreListData("all", 0)
	if err != nil {
		http.Error(w, "failed to load chores", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "chore-list-all", choreData)
}

// ChoreManagePage renders the full chore management page.
func (h *TemplateHandler) ChoreManagePage(w http.ResponseWriter, r *http.Request) {
	data, err := h.buildDashboardData(r, "chores")
	if err != nil {
		http.Error(w, "failed to load data", http.StatusInternalServerError)
		return
	}

	manageData, err := h.buildChoreManageData()
	if err != nil {
		h.logger.Error("build chore manage data", "error", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}

	content, err := h.renderSection("chore-manage-content", manageData)
	if err != nil {
		h.logger.Error("render chore manage content", "error", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	data["Content"] = content

	h.render(w, "layout.html", data)
}

// ChoreManagePartial renders the manage section partial.
func (h *TemplateHandler) ChoreManagePartial(w http.ResponseWriter, r *http.Request) {
	manageData, err := h.buildChoreManageData()
	if err != nil {
		http.Error(w, "failed to load data", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "chore-manage-content", manageData)
}

// ChoreAreaList renders the area list partial.
func (h *TemplateHandler) ChoreAreaList(w http.ResponseWriter, r *http.Request) {
	areas, err := h.choreStore.ListAreas()
	if err != nil {
		http.Error(w, "failed to load areas", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "chore-area-list", map[string]any{"Areas": areas})
}

// ChoreAreaCreate handles POST to create a new area.
func (h *TemplateHandler) ChoreAreaCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		h.renderToast(w, "error", "Area name is required")
		return
	}

	area, err := h.choreStore.CreateArea(name, 0)
	if err != nil {
		h.logger.Error("create area", "error", err)
		http.Error(w, "failed to create area", http.StatusInternalServerError)
		return
	}

	h.broadcast(websocket.NewMessage("chore_area", "created", area.ID, nil))

	h.renderToast(w, "success", "Area created")
	areas, _ := h.choreStore.ListAreas()
	h.renderPartial(w, "chore-area-list", map[string]any{"Areas": areas})
}

// ChoreAreaUpdate handles PUT to rename an area.
func (h *TemplateHandler) ChoreAreaUpdate(w http.ResponseWriter, r *http.Request) {
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
	if name == "" {
		h.renderToast(w, "error", "Area name is required")
		return
	}

	area, err := h.choreStore.GetAreaByID(id)
	if err != nil || area == nil {
		http.Error(w, "area not found", http.StatusNotFound)
		return
	}

	if _, err := h.choreStore.UpdateArea(id, name, area.SortOrder); err != nil {
		http.Error(w, "failed to update area", http.StatusInternalServerError)
		return
	}

	h.broadcast(websocket.NewMessage("chore_area", "updated", id, nil))

	h.renderToast(w, "success", "Area updated")
	areas, _ := h.choreStore.ListAreas()
	h.renderPartial(w, "chore-area-list", map[string]any{"Areas": areas})
}

// ChoreAreaDelete handles DELETE for an area.
func (h *TemplateHandler) ChoreAreaDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := h.choreStore.DeleteArea(id); err != nil {
		http.Error(w, "failed to delete area", http.StatusInternalServerError)
		return
	}

	h.broadcast(websocket.NewMessage("chore_area", "deleted", id, nil))

	h.renderToast(w, "success", "Area deleted")
	areas, _ := h.choreStore.ListAreas()
	h.renderPartial(w, "chore-area-list", map[string]any{"Areas": areas})
}

// --- Chore helper methods ---

type memberChoreSummary struct {
	MemberName  string
	MemberEmoji string
	MemberColor string
	Total       int
	Done        int
}

func (h *TemplateHandler) buildChoreListData(filter string, filterID int64) (map[string]any, error) {
	chores, err := h.choreStore.List()
	if err != nil {
		return nil, fmt.Errorf("list chores: %w", err)
	}

	members, err := h.store.List()
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}

	areas, err := h.choreStore.ListAreas()
	if err != nil {
		return nil, fmt.Errorf("list areas: %w", err)
	}

	memberMap := make(map[int64]model.FamilyMember)
	for _, m := range members {
		memberMap[m.ID] = m
	}
	areaMap := make(map[int64]model.ChoreArea)
	for _, a := range areas {
		areaMap[a.ID] = a
	}

	today := time.Now()
	var choreList []chore.ChoreWithStatus

	for _, c := range chores {
		last, _ := h.choreStore.LastCompletionForChore(c.ID)
		var lastTime *time.Time
		if last != nil {
			lastTime = &last.CompletedAt
		}
		status, dueDate := chore.ComputeStatus(c, lastTime, today)

		cws := chore.ChoreWithStatus{
			Chore:          c,
			Status:         status,
			DueDate:        dueDate,
			LastCompletion: lastTime,
		}

		if c.AreaID != nil {
			if a, ok := areaMap[*c.AreaID]; ok {
				cws.AreaName = a.Name
			}
		}
		if c.AssignedTo != nil {
			if m, ok := memberMap[*c.AssignedTo]; ok {
				cws.MemberName = m.Name
				cws.MemberColor = m.Color
				cws.MemberEmoji = m.AvatarEmoji
			}
		}

		choreList = append(choreList, cws)
	}

	// Sort: overdue > pending > completed > not_due
	sort.Slice(choreList, func(i, j int) bool {
		return statusPriority(choreList[i].Status) < statusPriority(choreList[j].Status)
	})

	// Group by person
	type personGroup struct {
		MemberName  string
		MemberEmoji string
		MemberColor string
		MemberID    int64
		Chores      []chore.ChoreWithStatus
		Total       int
		Done        int
	}
	personGroups := make(map[int64]*personGroup)
	var unassignedGroup *personGroup

	for _, c := range choreList {
		if c.AssignedTo != nil {
			mid := *c.AssignedTo
			if _, ok := personGroups[mid]; !ok {
				m := memberMap[mid]
				personGroups[mid] = &personGroup{
					MemberName:  m.Name,
					MemberEmoji: m.AvatarEmoji,
					MemberColor: m.Color,
					MemberID:    mid,
				}
			}
			pg := personGroups[mid]
			pg.Chores = append(pg.Chores, c)
			pg.Total++
			if c.Status == chore.StatusCompleted {
				pg.Done++
			}
		} else {
			if unassignedGroup == nil {
				unassignedGroup = &personGroup{MemberName: "Unassigned", MemberEmoji: "?"}
			}
			unassignedGroup.Chores = append(unassignedGroup.Chores, c)
			unassignedGroup.Total++
			if c.Status == chore.StatusCompleted {
				unassignedGroup.Done++
			}
		}
	}

	// Sort person groups by member order
	var personList []personGroup
	for _, m := range members {
		if pg, ok := personGroups[m.ID]; ok {
			personList = append(personList, *pg)
		}
	}
	if unassignedGroup != nil {
		personList = append(personList, *unassignedGroup)
	}

	// Group by area
	type areaGroup struct {
		AreaName string
		AreaID   int64
		Chores   []chore.ChoreWithStatus
	}
	areaGroups := make(map[int64]*areaGroup)
	var noAreaGroup *areaGroup

	for _, c := range choreList {
		if c.AreaID != nil {
			aid := *c.AreaID
			if _, ok := areaGroups[aid]; !ok {
				a := areaMap[aid]
				areaGroups[aid] = &areaGroup{AreaName: a.Name, AreaID: aid}
			}
			areaGroups[aid].Chores = append(areaGroups[aid].Chores, c)
		} else {
			if noAreaGroup == nil {
				noAreaGroup = &areaGroup{AreaName: "No Area"}
			}
			noAreaGroup.Chores = append(noAreaGroup.Chores, c)
		}
	}

	var areaList []areaGroup
	for _, a := range areas {
		if ag, ok := areaGroups[a.ID]; ok {
			areaList = append(areaList, *ag)
		}
	}
	if noAreaGroup != nil {
		areaList = append(areaList, *noAreaGroup)
	}

	return map[string]any{
		"Chores":       choreList,
		"PersonGroups": personList,
		"AreaGroups":   areaList,
		"Members":      members,
		"Areas":        areas,
		"Filter":       filter,
	}, nil
}

func (h *TemplateHandler) buildChoreManageData() (map[string]any, error) {
	chores, err := h.choreStore.List()
	if err != nil {
		return nil, fmt.Errorf("list chores: %w", err)
	}

	areas, err := h.choreStore.ListAreas()
	if err != nil {
		return nil, fmt.Errorf("list areas: %w", err)
	}

	members, _ := h.store.List()

	areaMap := make(map[int64]model.ChoreArea)
	for _, a := range areas {
		areaMap[a.ID] = a
	}
	memberMap := make(map[int64]model.FamilyMember)
	for _, m := range members {
		memberMap[m.ID] = m
	}

	type choreDisplay struct {
		model.Chore
		AreaName   string
		MemberName string
	}
	var choreDisplays []choreDisplay
	for _, c := range chores {
		cd := choreDisplay{Chore: c}
		if c.AreaID != nil {
			if a, ok := areaMap[*c.AreaID]; ok {
				cd.AreaName = a.Name
			}
		}
		if c.AssignedTo != nil {
			if m, ok := memberMap[*c.AssignedTo]; ok {
				cd.MemberName = m.Name
			}
		}
		choreDisplays = append(choreDisplays, cd)
	}

	return map[string]any{
		"Chores":  choreDisplays,
		"Areas":   areas,
		"Members": members,
	}, nil
}

func (h *TemplateHandler) buildChoreSummaryData() ([]memberChoreSummary, error) {
	chores, err := h.choreStore.List()
	if err != nil {
		return nil, fmt.Errorf("list chores: %w", err)
	}

	members, err := h.store.List()
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}

	today := time.Now()
	memberMap := make(map[int64]model.FamilyMember)
	for _, m := range members {
		memberMap[m.ID] = m
	}

	// Count per member
	type counts struct {
		total int
		done  int
	}
	memberCounts := make(map[int64]*counts)

	for _, c := range chores {
		if c.AssignedTo == nil {
			continue
		}
		mid := *c.AssignedTo
		if _, ok := memberCounts[mid]; !ok {
			memberCounts[mid] = &counts{}
		}
		mc := memberCounts[mid]
		mc.total++

		last, _ := h.choreStore.LastCompletionForChore(c.ID)
		var lastTime *time.Time
		if last != nil {
			lastTime = &last.CompletedAt
		}
		status, _ := chore.ComputeStatus(c, lastTime, today)
		if status == chore.StatusCompleted {
			mc.done++
		}
	}

	var summaries []memberChoreSummary
	for _, m := range members {
		if mc, ok := memberCounts[m.ID]; ok {
			summaries = append(summaries, memberChoreSummary{
				MemberName:  m.Name,
				MemberEmoji: m.AvatarEmoji,
				MemberColor: m.Color,
				Total:       mc.total,
				Done:        mc.done,
			})
		}
	}

	return summaries, nil
}

func statusPriority(s chore.Status) int {
	switch s {
	case chore.StatusOverdue:
		return 0
	case chore.StatusPending:
		return 1
	case chore.StatusCompleted:
		return 2
	case chore.StatusNotDue:
		return 3
	default:
		return 4
	}
}

// GroceryPage renders the full grocery page inside the dashboard layout.
func (h *TemplateHandler) GroceryPage(w http.ResponseWriter, r *http.Request) {
	data, err := h.buildDashboardData(r, "grocery")
	if err != nil {
		http.Error(w, "failed to load data", http.StatusInternalServerError)
		return
	}

	groceryData, err := h.buildGroceryListData()
	if err != nil {
		h.logger.Error("build grocery list", "error", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}

	content, err := h.renderSection("grocery-content", groceryData)
	if err != nil {
		h.logger.Error("render grocery content", "error", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	data["Content"] = content

	h.render(w, "layout.html", data)
}

// GroceryPartial renders the grocery section for HTMX swap.
func (h *TemplateHandler) GroceryPartial(w http.ResponseWriter, r *http.Request) {
	groceryData, err := h.buildGroceryListData()
	if err != nil {
		h.logger.Error("build grocery list", "error", err)
		http.Error(w, "failed to load grocery data", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "grocery-content", groceryData)
}

// GroceryItemList renders the item list partial.
func (h *TemplateHandler) GroceryItemList(w http.ResponseWriter, r *http.Request) {
	groceryData, err := h.buildGroceryListData()
	if err != nil {
		http.Error(w, "failed to load grocery data", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "grocery-item-list", groceryData)
}

// GroceryItemAdd handles POST quick-add form submission.
func (h *TemplateHandler) GroceryItemAdd(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		h.renderToast(w, "error", "Item name is required")
		return
	}

	list, err := h.groceryStore.GetDefaultList()
	if err != nil || list == nil {
		http.Error(w, "failed to get list", http.StatusInternalServerError)
		return
	}

	activeUserID := h.activeUserFromCookie(r)
	var addedBy *int64
	if activeUserID > 0 {
		addedBy = &activeUserID
	}

	cat := grocery.Categorize(name)

	item, err := h.groceryStore.CreateItem(list.ID, name, "", "", "", cat, addedBy)
	if err != nil {
		h.logger.Error("create grocery item", "error", err)
		http.Error(w, "failed to create item", http.StatusInternalServerError)
		return
	}

	h.broadcast(websocket.NewMessage("grocery_item", "created", item.ID, map[string]any{"list_id": list.ID}))

	// Fire push notification for grocery addition (exclude the adding user)
	if h.pushScheduler != nil {
		userID := auth.UserID(r.Context())
		householdID := auth.HouseholdID(r.Context())
		go h.pushScheduler.SendGroceryNotification(householdID, userID, name)
	}

	groceryData, err := h.buildGroceryListData()
	if err != nil {
		http.Error(w, "failed to load grocery data", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "grocery-item-list", groceryData)
}

// GroceryItemToggle handles POST to toggle checked state.
func (h *TemplateHandler) GroceryItemToggle(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	activeUserID := h.activeUserFromCookie(r)
	var checkedBy *int64
	if activeUserID > 0 {
		checkedBy = &activeUserID
	}

	if _, err := h.groceryStore.ToggleChecked(id, checkedBy); err != nil {
		h.logger.Error("toggle grocery item", "error", err)
		http.Error(w, "failed to toggle item", http.StatusInternalServerError)
		return
	}

	h.broadcast(websocket.NewMessage("grocery_item", "checked", id, nil))

	groceryData, err := h.buildGroceryListData()
	if err != nil {
		http.Error(w, "failed to load grocery data", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "grocery-item-list", groceryData)
}

// GroceryItemDelete handles DELETE for a grocery item.
func (h *TemplateHandler) GroceryItemDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := h.groceryStore.DeleteItem(id); err != nil {
		http.Error(w, "failed to delete item", http.StatusInternalServerError)
		return
	}

	h.broadcast(websocket.NewMessage("grocery_item", "deleted", id, nil))

	groceryData, err := h.buildGroceryListData()
	if err != nil {
		http.Error(w, "failed to load grocery data", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "grocery-item-list", groceryData)
}

// GroceryClearChecked handles POST to clear all checked items.
func (h *TemplateHandler) GroceryClearChecked(w http.ResponseWriter, r *http.Request) {
	list, err := h.groceryStore.GetDefaultList()
	if err != nil || list == nil {
		http.Error(w, "failed to get list", http.StatusInternalServerError)
		return
	}

	count, err := h.groceryStore.ClearChecked(list.ID)
	if err != nil {
		http.Error(w, "failed to clear checked", http.StatusInternalServerError)
		return
	}

	h.broadcast(websocket.NewMessage("grocery_item", "cleared", 0, map[string]any{"list_id": list.ID}))

	h.renderToast(w, "success", fmt.Sprintf("Cleared %d item(s)", count))

	groceryData, err := h.buildGroceryListData()
	if err != nil {
		http.Error(w, "failed to load grocery data", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "grocery-item-list", groceryData)
}

// GroceryItemEditForm renders the edit form in the modal.
func (h *TemplateHandler) GroceryItemEditForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	item, err := h.groceryStore.GetItemByID(id)
	if err != nil || item == nil {
		http.Error(w, "item not found", http.StatusNotFound)
		return
	}

	categories, _ := h.groceryStore.ListCategories()

	h.renderPartial(w, "grocery-edit-form", map[string]any{
		"ID":         item.ID,
		"Name":       item.Name,
		"Quantity":   item.Quantity,
		"Unit":       item.Unit,
		"Notes":      item.Notes,
		"Category":   item.Category,
		"Categories": categories,
	})
}

// GroceryItemUpdate handles PUT form submission to update a grocery item.
func (h *TemplateHandler) GroceryItemUpdate(w http.ResponseWriter, r *http.Request) {
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
	if name == "" {
		h.renderToast(w, "error", "Item name is required")
		return
	}

	quantity := r.FormValue("quantity")
	unit := r.FormValue("unit")
	notes := r.FormValue("notes")
	category := r.FormValue("category")

	if category == "" {
		category = "Other"
	}

	if _, err := h.groceryStore.UpdateItem(id, name, quantity, unit, notes, category); err != nil {
		h.logger.Error("update grocery item", "error", err)
		http.Error(w, "failed to update item", http.StatusInternalServerError)
		return
	}

	h.broadcast(websocket.NewMessage("grocery_item", "updated", id, nil))

	w.Header().Set("HX-Trigger", "closeGroceryModal")
	h.renderToast(w, "success", "Item updated")

	groceryData, err := h.buildGroceryListData()
	if err != nil {
		http.Error(w, "failed to load grocery data", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "grocery-item-list", groceryData)
}

// --- Grocery helper methods ---

type categoryGroup struct {
	CategoryName string
	Items        []model.GroceryItem
}

func (h *TemplateHandler) buildGroceryListData() (map[string]any, error) {
	list, err := h.groceryStore.GetDefaultList()
	if err != nil {
		return nil, fmt.Errorf("get default list: %w", err)
	}
	if list == nil {
		return map[string]any{
			"List":           nil,
			"CategoryGroups": nil,
			"CheckedItems":   nil,
			"UncheckedCount": 0,
			"Categories":     nil,
		}, nil
	}

	items, err := h.groceryStore.ListItemsByList(list.ID)
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}

	categories, err := h.groceryStore.ListCategories()
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}

	// Build category sort order map
	catOrder := make(map[string]int)
	for _, c := range categories {
		catOrder[c.Name] = c.SortOrder
	}

	// Separate unchecked and checked
	var unchecked, checked []model.GroceryItem
	for _, item := range items {
		if item.Checked {
			checked = append(checked, item)
		} else {
			unchecked = append(unchecked, item)
		}
	}

	// Group unchecked by category
	groupMap := make(map[string][]model.GroceryItem)
	for _, item := range unchecked {
		groupMap[item.Category] = append(groupMap[item.Category], item)
	}

	// Sort groups by category sort order
	var groups []categoryGroup
	for _, cat := range categories {
		if items, ok := groupMap[cat.Name]; ok {
			groups = append(groups, categoryGroup{
				CategoryName: cat.Name,
				Items:        items,
			})
		}
	}
	// Add any categories not in seed data
	for catName, items := range groupMap {
		if _, ok := catOrder[catName]; !ok {
			groups = append(groups, categoryGroup{
				CategoryName: catName,
				Items:        items,
			})
		}
	}

	return map[string]any{
		"List":           list,
		"CategoryGroups": groups,
		"CheckedItems":   checked,
		"UncheckedCount": len(unchecked),
		"Categories":     categories,
	}, nil
}

func (h *TemplateHandler) buildGrocerySummaryData() (map[string]any, error) {
	list, err := h.groceryStore.GetDefaultList()
	if err != nil {
		return nil, fmt.Errorf("get default list: %w", err)
	}
	if list == nil {
		return map[string]any{"UncheckedCount": 0}, nil
	}

	count, err := h.groceryStore.CountUnchecked(list.ID)
	if err != nil {
		return nil, fmt.Errorf("count unchecked: %w", err)
	}

	return map[string]any{"UncheckedCount": count}, nil
}

// --- Notes handlers ---

// NotesPage renders the full notes page inside the dashboard layout.
func (h *TemplateHandler) NotesPage(w http.ResponseWriter, r *http.Request) {
	data, err := h.buildDashboardData(r, "notes")
	if err != nil {
		http.Error(w, "failed to load data", http.StatusInternalServerError)
		return
	}

	noteData, err := h.buildNoteListData()
	if err != nil {
		h.logger.Error("build note list", "error", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	noteData["Members"] = data["Members"]

	content, err := h.renderSection("notes-content", noteData)
	if err != nil {
		h.logger.Error("render notes content", "error", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	data["Content"] = content

	h.render(w, "layout.html", data)
}

// NotesPartial renders the notes section for HTMX swap.
func (h *TemplateHandler) NotesPartial(w http.ResponseWriter, r *http.Request) {
	noteData, err := h.buildNoteListData()
	if err != nil {
		h.logger.Error("build note list", "error", err)
		http.Error(w, "failed to load notes data", http.StatusInternalServerError)
		return
	}
	members, _ := h.store.List()
	noteData["Members"] = members
	h.renderPartial(w, "notes-content", noteData)
}

// NoteList renders the note list partial.
func (h *TemplateHandler) NoteList(w http.ResponseWriter, r *http.Request) {
	noteData, err := h.buildNoteListData()
	if err != nil {
		http.Error(w, "failed to load notes data", http.StatusInternalServerError)
		return
	}
	members, _ := h.store.List()
	noteData["Members"] = members
	h.renderPartial(w, "note-list", noteData)
}

// NoteNewForm renders the new note form in the modal.
func (h *TemplateHandler) NoteNewForm(w http.ResponseWriter, r *http.Request) {
	members, _ := h.store.List()
	data := map[string]any{
		"Members": members,
	}
	h.renderPartial(w, "note-form", data)
}

// NoteEditForm renders the edit note form in the modal.
func (h *TemplateHandler) NoteEditForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	note, err := h.noteStore.GetByID(id)
	if err != nil || note == nil {
		http.Error(w, "note not found", http.StatusNotFound)
		return
	}

	members, _ := h.store.List()
	data := map[string]any{
		"Note":    note,
		"Members": members,
	}
	h.renderPartial(w, "note-edit-form", data)
}

// NoteCreate handles POST form submission to create a note.
func (h *TemplateHandler) NoteCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		h.renderToast(w, "error", "Title is required")
		return
	}

	body := r.FormValue("body")
	priority := r.FormValue("priority")
	if priority == "" {
		priority = "normal"
	}

	pinned := r.FormValue("pinned") == "on" || r.FormValue("pinned") == "true"

	activeUserID := h.activeUserFromCookie(r)
	var authorID *int64
	if activeUserID > 0 {
		authorID = &activeUserID
	}

	var expiresAt *time.Time
	if exp := r.FormValue("expires_at"); exp != "" {
		t, err := time.Parse("2006-01-02", exp)
		if err == nil {
			// Set to end of day UTC
			t = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			expiresAt = &t
		}
	}

	note, err := h.noteStore.Create(title, body, authorID, pinned, priority, expiresAt)
	if err != nil {
		h.logger.Error("create note", "error", err)
		http.Error(w, "failed to create note", http.StatusInternalServerError)
		return
	}

	h.broadcast(websocket.NewMessage("note", "created", note.ID, nil))

	w.Header().Set("HX-Trigger", "closeNoteModal")

	noteData, err := h.buildNoteListData()
	if err != nil {
		http.Error(w, "failed to load notes data", http.StatusInternalServerError)
		return
	}
	members, _ := h.store.List()
	noteData["Members"] = members
	h.renderPartial(w, "note-list", noteData)
}

// NoteUpdate handles PUT form submission to update a note.
func (h *TemplateHandler) NoteUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		h.renderToast(w, "error", "Title is required")
		return
	}

	body := r.FormValue("body")
	priority := r.FormValue("priority")
	if priority == "" {
		priority = "normal"
	}

	pinned := r.FormValue("pinned") == "on" || r.FormValue("pinned") == "true"

	// Preserve existing author or use active user
	existing, _ := h.noteStore.GetByID(id)
	var authorID *int64
	if existing != nil {
		authorID = existing.AuthorID
	}

	var expiresAt *time.Time
	if exp := r.FormValue("expires_at"); exp != "" {
		t, err := time.Parse("2006-01-02", exp)
		if err == nil {
			t = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			expiresAt = &t
		}
	}

	note, err := h.noteStore.Update(id, title, body, authorID, pinned, priority, expiresAt)
	if err != nil {
		h.logger.Error("update note", "error", err)
		http.Error(w, "failed to update note", http.StatusInternalServerError)
		return
	}

	h.broadcast(websocket.NewMessage("note", "updated", note.ID, nil))

	w.Header().Set("HX-Trigger", "closeNoteModal")

	noteData, err := h.buildNoteListData()
	if err != nil {
		http.Error(w, "failed to load notes data", http.StatusInternalServerError)
		return
	}
	members, _ := h.store.List()
	noteData["Members"] = members
	h.renderPartial(w, "note-list", noteData)
}

// NoteDelete handles DELETE to remove a note.
func (h *TemplateHandler) NoteDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := h.noteStore.Delete(id); err != nil {
		h.logger.Error("delete note", "error", err)
		http.Error(w, "failed to delete note", http.StatusInternalServerError)
		return
	}

	h.broadcast(websocket.NewMessage("note", "deleted", id, nil))

	w.Header().Set("HX-Trigger", "closeNoteModal")

	noteData, err := h.buildNoteListData()
	if err != nil {
		http.Error(w, "failed to load notes data", http.StatusInternalServerError)
		return
	}
	members, _ := h.store.List()
	noteData["Members"] = members
	h.renderPartial(w, "note-list", noteData)
}

// NoteTogglePin handles POST to toggle a note's pinned status.
func (h *TemplateHandler) NoteTogglePin(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	note, err := h.noteStore.TogglePinned(id)
	if err != nil || note == nil {
		http.Error(w, "failed to toggle pin", http.StatusInternalServerError)
		return
	}

	h.broadcast(websocket.NewMessage("note", "pinned", id, nil))

	noteData, err := h.buildNoteListData()
	if err != nil {
		http.Error(w, "failed to load notes data", http.StatusInternalServerError)
		return
	}
	members, _ := h.store.List()
	noteData["Members"] = members
	h.renderPartial(w, "note-list", noteData)
}

// buildNoteListData assembles note data for the notes section.
func (h *TemplateHandler) buildNoteListData() (map[string]any, error) {
	notes, err := h.noteStore.List()
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}

	var pinned, unpinned []model.Note
	for _, n := range notes {
		if n.Pinned {
			pinned = append(pinned, n)
		} else {
			unpinned = append(unpinned, n)
		}
	}

	return map[string]any{
		"PinnedNotes":   pinned,
		"UnpinnedNotes": unpinned,
		"NoteCount":     len(notes),
	}, nil
}

// buildNoteSummaryData returns pinned notes for the dashboard widget.
func (h *TemplateHandler) buildNoteSummaryData() (map[string]any, error) {
	pinned, err := h.noteStore.ListPinned()
	if err != nil {
		return nil, fmt.Errorf("list pinned notes: %w", err)
	}

	total, err := h.noteStore.List()
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}

	return map[string]any{
		"PinnedNotes": pinned,
		"NoteCount":   len(total),
	}, nil
}

// --- Rewards handlers ---

// RewardsPage renders the full rewards page inside the dashboard layout.
func (h *TemplateHandler) RewardsPage(w http.ResponseWriter, r *http.Request) {
	data, err := h.buildDashboardData(r, "chores")
	if err != nil {
		http.Error(w, "failed to load data", http.StatusInternalServerError)
		return
	}

	rewardsData, err := h.buildRewardsData(r)
	if err != nil {
		h.logger.Error("build rewards data", "error", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}

	content, err := h.renderSection("rewards-content", rewardsData)
	if err != nil {
		h.logger.Error("render rewards content", "error", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	data["Content"] = content

	h.render(w, "layout.html", data)
}

// RewardsPartial renders the rewards section for HTMX swap.
func (h *TemplateHandler) RewardsPartial(w http.ResponseWriter, r *http.Request) {
	rewardsData, err := h.buildRewardsData(r)
	if err != nil {
		h.logger.Error("build rewards data", "error", err)
		http.Error(w, "failed to load rewards data", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "rewards-content", rewardsData)
}

// RewardsList renders the rewards list partial.
func (h *TemplateHandler) RewardsList(w http.ResponseWriter, r *http.Request) {
	rewardsData, err := h.buildRewardsData(r)
	if err != nil {
		http.Error(w, "failed to load rewards data", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "rewards-list", rewardsData)
}

// RewardRedeem handles POST to redeem a reward.
func (h *TemplateHandler) RewardRedeem(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	reward, err := h.rewardStore.GetByID(id)
	if err != nil || reward == nil {
		http.Error(w, "reward not found", http.StatusNotFound)
		return
	}
	if !reward.Active {
		h.renderToast(w, "error", "Reward is not active")
		return
	}

	activeUserID := h.activeUserFromCookie(r)
	var redeemedBy *int64
	if activeUserID > 0 {
		redeemedBy = &activeUserID
	}

	// Check balance
	if redeemedBy != nil {
		balance, err := h.rewardStore.GetPointBalance(*redeemedBy)
		if err != nil {
			http.Error(w, "failed to check balance", http.StatusInternalServerError)
			return
		}
		if balance.Balance < reward.PointCost {
			h.renderToast(w, "error", "Not enough points!")
			return
		}
	}

	if _, err := h.rewardStore.Redeem(id, redeemedBy, reward.PointCost); err != nil {
		h.logger.Error("redeem reward", "error", err)
		http.Error(w, "failed to redeem reward", http.StatusInternalServerError)
		return
	}

	h.broadcast(websocket.NewMessage("reward", "redeemed", id, nil))

	h.renderToast(w, "success", fmt.Sprintf("Redeemed: %s!", reward.Title))

	rewardsData, err := h.buildRewardsData(r)
	if err != nil {
		http.Error(w, "failed to load rewards data", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "rewards-list", rewardsData)
}

// --- Reward management (settings) handlers ---

// RewardManagePartial renders the reward management list for settings.
func (h *TemplateHandler) RewardManagePartial(w http.ResponseWriter, r *http.Request) {
	rewards, err := h.rewardStore.List()
	if err != nil {
		http.Error(w, "failed to load rewards", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "reward-manage-list", map[string]any{"Rewards": rewards})
}

// RewardNewForm renders the new reward form.
func (h *TemplateHandler) RewardNewForm(w http.ResponseWriter, r *http.Request) {
	h.renderPartial(w, "reward-form", nil)
}

// RewardEditForm renders the edit reward form.
func (h *TemplateHandler) RewardEditForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	reward, err := h.rewardStore.GetByID(id)
	if err != nil || reward == nil {
		http.Error(w, "reward not found", http.StatusNotFound)
		return
	}

	h.renderPartial(w, "reward-edit-form", map[string]any{"Reward": reward})
}

// RewardCreate handles POST to create a reward.
func (h *TemplateHandler) RewardCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		h.renderToast(w, "error", "Title is required")
		return
	}

	description := r.FormValue("description")
	pointCost, _ := strconv.Atoi(r.FormValue("point_cost"))
	active := r.FormValue("active") == "on" || r.FormValue("active") == "true"

	reward, err := h.rewardStore.Create(title, description, pointCost, active)
	if err != nil {
		h.logger.Error("create reward", "error", err)
		http.Error(w, "failed to create reward", http.StatusInternalServerError)
		return
	}

	h.broadcast(websocket.NewMessage("reward", "created", reward.ID, nil))

	w.Header().Set("HX-Trigger", "closeRewardModal")

	rewards, _ := h.rewardStore.List()
	h.renderPartial(w, "reward-manage-list", map[string]any{"Rewards": rewards})
}

// RewardUpdate handles PUT to update a reward.
func (h *TemplateHandler) RewardUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		h.renderToast(w, "error", "Title is required")
		return
	}

	description := r.FormValue("description")
	pointCost, _ := strconv.Atoi(r.FormValue("point_cost"))
	active := r.FormValue("active") == "on" || r.FormValue("active") == "true"

	reward, err := h.rewardStore.Update(id, title, description, pointCost, active)
	if err != nil {
		h.logger.Error("update reward", "error", err)
		http.Error(w, "failed to update reward", http.StatusInternalServerError)
		return
	}

	h.broadcast(websocket.NewMessage("reward", "updated", reward.ID, nil))

	w.Header().Set("HX-Trigger", "closeRewardModal")

	rewards, _ := h.rewardStore.List()
	h.renderPartial(w, "reward-manage-list", map[string]any{"Rewards": rewards})
}

// RewardDelete handles DELETE to remove a reward.
func (h *TemplateHandler) RewardDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := h.rewardStore.Delete(id); err != nil {
		h.logger.Error("delete reward", "error", err)
		http.Error(w, "failed to delete reward", http.StatusInternalServerError)
		return
	}

	h.broadcast(websocket.NewMessage("reward", "deleted", id, nil))

	w.Header().Set("HX-Trigger", "closeRewardModal")

	rewards, _ := h.rewardStore.List()
	h.renderPartial(w, "reward-manage-list", map[string]any{"Rewards": rewards})
}

// buildRewardsData assembles data for the rewards page.
func (h *TemplateHandler) buildRewardsData(r *http.Request) (map[string]any, error) {
	rewards, err := h.rewardStore.ListActive()
	if err != nil {
		return nil, fmt.Errorf("list active rewards: %w", err)
	}

	members, err := h.store.List()
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}

	activeUserID := h.activeUserFromCookie(r)
	var userBalance *model.PointBalance
	if activeUserID > 0 {
		userBalance, _ = h.rewardStore.GetPointBalance(activeUserID)
	}

	balances, _ := h.rewardStore.GetAllPointBalances()

	return map[string]any{
		"Rewards":      rewards,
		"Members":      members,
		"UserBalance":  userBalance,
		"Balances":     balances,
		"ActiveUserID": activeUserID,
	}, nil
}

// buildLeaderboardData returns data for the dashboard leaderboard widget.
func (h *TemplateHandler) buildLeaderboardData() (map[string]any, error) {
	enabled, _ := h.settingsStore.Get("rewards_leaderboard_enabled")

	balances, err := h.rewardStore.GetAllPointBalances()
	if err != nil {
		return nil, fmt.Errorf("get all balances: %w", err)
	}

	return map[string]any{
		"Enabled":  enabled == "true",
		"Balances": balances,
	}, nil
}

// SettingsPartial renders the settings section for HTMX swap.
func (h *TemplateHandler) SettingsPartial(w http.ResponseWriter, r *http.Request) {
	h.renderPartial(w, "settings-content", nil)
}

// KioskSettingsPartial renders the kiosk settings form for HTMX swap.
func (h *TemplateHandler) KioskSettingsPartial(w http.ResponseWriter, r *http.Request) {
	settings, err := h.settingsStore.GetKioskSettings()
	if err != nil {
		h.logger.Error("get kiosk settings", "error", err)
		http.Error(w, "failed to load settings", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "kiosk-settings-form", settings)
}

// KioskSettingsUpdate handles PUT form submission for kiosk settings.
func (h *TemplateHandler) KioskSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	settings := map[string]string{
		"idle_timeout_minutes": r.FormValue("idle_timeout_minutes"),
		"quiet_hours_enabled":  r.FormValue("quiet_hours_enabled"),
		"quiet_hours_start":    r.FormValue("quiet_hours_start"),
		"quiet_hours_end":      r.FormValue("quiet_hours_end"),
		"burn_in_prevention":   r.FormValue("burn_in_prevention"),
	}

	// Normalize checkbox/toggle values
	if settings["quiet_hours_enabled"] == "" || settings["quiet_hours_enabled"] == "on" {
		if r.FormValue("quiet_hours_enabled") == "on" {
			settings["quiet_hours_enabled"] = "true"
		} else if r.FormValue("quiet_hours_enabled") == "" {
			settings["quiet_hours_enabled"] = "false"
		}
	}
	if settings["burn_in_prevention"] == "" || settings["burn_in_prevention"] == "on" {
		if r.FormValue("burn_in_prevention") == "on" {
			settings["burn_in_prevention"] = "true"
		} else if r.FormValue("burn_in_prevention") == "" {
			settings["burn_in_prevention"] = "false"
		}
	}

	for key, value := range settings {
		if err := h.settingsStore.Set(key, value); err != nil {
			h.logger.Error("set setting", "key", key, "error", err)
			h.renderToast(w, "error", "Failed to save settings")
			return
		}
	}

	h.broadcast(websocket.NewMessage("settings", "updated", 0, nil))

	updated, err := h.settingsStore.GetKioskSettings()
	if err != nil {
		h.logger.Error("get kiosk settings", "error", err)
		http.Error(w, "failed to load settings", http.StatusInternalServerError)
		return
	}

	h.renderToast(w, "success", "Kiosk settings saved")
	h.renderPartial(w, "kiosk-settings-form", updated)
}

// WeatherSettingsPartial renders the weather settings form for HTMX swap.
func (h *TemplateHandler) WeatherSettingsPartial(w http.ResponseWriter, r *http.Request) {
	settings, err := h.settingsStore.GetWeatherSettings()
	if err != nil {
		h.logger.Error("get weather settings", "error", err)
		http.Error(w, "failed to load settings", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "weather-settings-form", settings)
}

// WeatherSettingsUpdate handles PUT form submission for weather settings.
func (h *TemplateHandler) WeatherSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	settings := map[string]string{
		"weather_latitude":  strings.TrimSpace(r.FormValue("weather_latitude")),
		"weather_longitude": strings.TrimSpace(r.FormValue("weather_longitude")),
		"weather_units":     r.FormValue("weather_units"),
	}

	// Validate lat/lon are valid floats (or empty)
	for _, key := range []string{"weather_latitude", "weather_longitude"} {
		if v := settings[key]; v != "" {
			if _, err := strconv.ParseFloat(v, 64); err != nil {
				h.renderToast(w, "error", fmt.Sprintf("Invalid %s value", key))
				return
			}
		}
	}

	// Validate units
	if settings["weather_units"] != "fahrenheit" && settings["weather_units"] != "celsius" {
		h.renderToast(w, "error", "Temperature unit must be fahrenheit or celsius")
		return
	}

	for key, value := range settings {
		if err := h.settingsStore.Set(key, value); err != nil {
			h.logger.Error("set setting", "key", key, "error", err)
			h.renderToast(w, "error", "Failed to save settings")
			return
		}
	}

	h.weatherSvc.UpdateConfig(weather.Config{
		Latitude:        settings["weather_latitude"],
		Longitude:       settings["weather_longitude"],
		TemperatureUnit: settings["weather_units"],
	})

	h.broadcast(websocket.NewMessage("settings", "updated", 0, nil))

	updated, err := h.settingsStore.GetWeatherSettings()
	if err != nil {
		h.logger.Error("get weather settings", "error", err)
		http.Error(w, "failed to load settings", http.StatusInternalServerError)
		return
	}

	h.renderToast(w, "success", "Weather settings saved")
	h.renderPartial(w, "weather-settings-form", updated)
}

// ThemeSettingsPartial renders the theme settings form for HTMX swap.
func (h *TemplateHandler) ThemeSettingsPartial(w http.ResponseWriter, r *http.Request) {
	settings, err := h.settingsStore.GetThemeSettings()
	if err != nil {
		h.logger.Error("get theme settings", "error", err)
		http.Error(w, "failed to load settings", http.StatusInternalServerError)
		return
	}
	h.renderPartial(w, "theme-settings-form", settings)
}

// ThemeSettingsUpdate handles PUT form submission for theme settings.
func (h *TemplateHandler) ThemeSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	mode := r.FormValue("theme_mode")
	validModes := map[string]bool{"manual": true, "system": true, "quiet-hours": true}
	if !validModes[mode] {
		h.renderToast(w, "error", "Invalid theme mode")
		return
	}

	validThemes := map[string]bool{"light": true, "dark": true, "sunny": true, "cozy": true, "grounded": true, "starfall": true, "silverglade": true, "seamist": true, "irondeep": true, "horsemark": true, "citadel": true}

	selected := r.FormValue("theme_selected")
	if selected != "" && !validThemes[selected] {
		h.renderToast(w, "error", "Invalid theme selection")
		return
	}

	light := r.FormValue("theme_light")
	if light != "" && !validThemes[light] {
		h.renderToast(w, "error", "Invalid light theme selection")
		return
	}

	dark := r.FormValue("theme_dark")
	if dark != "" && !validThemes[dark] {
		h.renderToast(w, "error", "Invalid dark theme selection")
		return
	}

	settings := map[string]string{
		"theme_mode":     mode,
		"theme_selected": selected,
		"theme_light":    light,
		"theme_dark":     dark,
	}

	for key, value := range settings {
		if err := h.settingsStore.Set(key, value); err != nil {
			h.logger.Error("set setting", "key", key, "error", err)
			h.renderToast(w, "error", "Failed to save settings")
			return
		}
	}

	h.broadcast(websocket.NewMessage("settings", "updated", 0, nil))

	updated, err := h.settingsStore.GetThemeSettings()
	if err != nil {
		h.logger.Error("get theme settings", "error", err)
		http.Error(w, "failed to load settings", http.StatusInternalServerError)
		return
	}

	h.renderToast(w, "success", "Theme settings saved")
	h.renderPartial(w, "theme-settings-form", updated)
}

// NextUpcomingEventPartial renders the next upcoming event for the idle screen.
func (h *TemplateHandler) NextUpcomingEventPartial(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	rangeEnd := now.Add(24 * time.Hour)

	events, err := h.expandEventsForRange(now, rangeEnd)
	if err != nil {
		h.logger.Error("expand events for idle", "error", err)
		h.renderPartial(w, "idle-next-event", nil)
		return
	}

	var nextEvent *expandedEvent
	for i := range events {
		if events[i].StartTime.After(now) && !events[i].AllDay {
			nextEvent = &events[i]
			break
		}
	}

	h.renderPartial(w, "idle-next-event", nextEvent)
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
		h.logger.Error("render family members content", "error", err)
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

	member, err := h.store.Create(name, color, avatarEmoji)
	if err != nil {
		http.Error(w, "failed to create family member", http.StatusInternalServerError)
		return
	}

	h.broadcast(websocket.NewMessage("family_member", "created", member.ID, nil))

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

	h.broadcast(websocket.NewMessage("family_member", "updated", id, nil))

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

	h.broadcast(websocket.NewMessage("family_member", "deleted", id, nil))

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
	ID             int64
	Title          string
	Color          string
	MemberEmoji    string
	TimeRange      string
	TopPx          int
	HeightPx       int
	IsRecurring    bool
	ParentID       int64
	OccurrenceDate string
}

type weekDayEvent struct {
	Title       string
	Color       string
	IsRecurring bool
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

// expandedEvent is a flattened event including virtual occurrences of recurring events.
type expandedEvent struct {
	model.CalendarEvent
	IsRecurring    bool
	ParentID       int64
	OccurrenceDate string // "2006-01-02" for identifying specific occurrences
}

// expandEventsForRange returns both non-recurring and expanded recurring events for a date range.
func (h *TemplateHandler) expandEventsForRange(rangeStart, rangeEnd time.Time) ([]expandedEvent, error) {
	// 1. Get non-recurring events
	nonRecurring, err := h.eventStore.ListByDateRange(rangeStart, rangeEnd)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}

	var results []expandedEvent
	for _, e := range nonRecurring {
		results = append(results, expandedEvent{
			CalendarEvent:  e,
			OccurrenceDate: e.StartTime.Format("2006-01-02"),
		})
	}

	// 2. Get recurring parent events
	recurring, err := h.eventStore.ListRecurring(rangeEnd)
	if err != nil {
		return nil, fmt.Errorf("list recurring: %w", err)
	}

	// 3. Expand each recurring event
	for _, parent := range recurring {
		rule, err := recurrence.Parse(parent.RecurrenceRule)
		if err != nil {
			h.logger.Error("skip recurring event", "event_id", parent.ID, "rule", parent.RecurrenceRule, "error", err)
			continue
		}

		occurrences := recurrence.Expand(rule, parent.StartTime, parent.EndTime, rangeStart, rangeEnd)

		// Get exceptions for this parent
		exceptions, err := h.eventStore.ListExceptions(parent.ID)
		if err != nil {
			return nil, fmt.Errorf("list exceptions for %d: %w", parent.ID, err)
		}

		// Build exception lookup by original start time (date-only key)
		excMap := make(map[string]model.CalendarEvent)
		for _, exc := range exceptions {
			if exc.OriginalStartTime != nil {
				key := exc.OriginalStartTime.Format("2006-01-02T15:04:05Z")
				excMap[key] = exc
			}
		}

		for _, occ := range occurrences {
			key := occ.Start.Format("2006-01-02T15:04:05Z")
			if exc, found := excMap[key]; found {
				if exc.Cancelled {
					continue // skip cancelled occurrence
				}
				// Use exception's data
				results = append(results, expandedEvent{
					CalendarEvent:  exc,
					IsRecurring:    true,
					ParentID:       parent.ID,
					OccurrenceDate: occ.Start.Format("2006-01-02"),
				})
			} else {
				// Virtual occurrence from parent data
				virtual := parent
				virtual.StartTime = occ.Start
				virtual.EndTime = occ.End
				results = append(results, expandedEvent{
					CalendarEvent:  virtual,
					IsRecurring:    true,
					ParentID:       parent.ID,
					OccurrenceDate: occ.Start.Format("2006-01-02"),
				})
			}
		}
	}

	// Sort: all-day first, then by start time
	sort.Slice(results, func(i, j int) bool {
		if results[i].AllDay != results[j].AllDay {
			return results[i].AllDay
		}
		return results[i].StartTime.Before(results[j].StartTime)
	})

	return results, nil
}

func (h *TemplateHandler) buildDayViewData(date time.Time) (map[string]any, error) {
	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	dayEnd := dayStart.Add(24 * time.Hour)

	events, err := h.expandEventsForRange(dayStart, dayEnd)
	if err != nil {
		return nil, fmt.Errorf("expand events: %w", err)
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
				ID:             e.ID,
				Title:          e.Title,
				Color:          color,
				MemberEmoji:    emoji,
				IsRecurring:    e.IsRecurring,
				ParentID:       e.ParentID,
				OccurrenceDate: e.OccurrenceDate,
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
			ID:             e.ID,
			Title:          e.Title,
			Color:          color,
			MemberEmoji:    emoji,
			TimeRange:      timeRange,
			TopPx:          topPx,
			HeightPx:       heightPx,
			IsRecurring:    e.IsRecurring,
			ParentID:       e.ParentID,
			OccurrenceDate: e.OccurrenceDate,
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
		"DateStr":      dayStart.Format("2006-01-02"),
		"DateLabel":    dayStart.Format("Monday, January 2, 2006"),
		"PrevDate":     dayStart.AddDate(0, 0, -1).Format("2006-01-02"),
		"NextDate":     dayStart.AddDate(0, 0, 1).Format("2006-01-02"),
		"IsToday":      isToday,
		"AllDayEvents": allDayEvents,
		"TimedEvents":  timedEvents,
		"Hours":        hours,
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

	events, err := h.expandEventsForRange(monday, sunday)
	if err != nil {
		return nil, fmt.Errorf("expand events: %w", err)
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
					Title:       e.Title,
					Color:       color,
					IsRecurring: e.IsRecurring,
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
		"Days":      days,
		"WeekLabel": weekLabel,
		"PrevWeek":  monday.AddDate(0, 0, -7).Format("2006-01-02"),
		"NextWeek":  monday.AddDate(0, 0, 7).Format("2006-01-02"),
		"TodayStr":  today.Format("2006-01-02"),
	}, nil
}

func (h *TemplateHandler) buildEventDetailData(event *model.CalendarEvent, isRecurring bool, parentID int64, occurrenceDate string) map[string]any {
	data := map[string]any{
		"ID":             event.ID,
		"Title":          event.Title,
		"Description":    event.Description,
		"Location":       event.Location,
		"AllDay":         event.AllDay,
		"IsRecurring":    isRecurring,
		"ParentID":       parentID,
		"OccurrenceDate": occurrenceDate,
	}

	if event.RecurrenceRule != "" {
		rule, err := recurrence.Parse(event.RecurrenceRule)
		if err == nil {
			data["RecurrenceDesc"] = rule.Describe()
		}
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

	if event.ReminderMinutes != nil {
		switch *event.ReminderMinutes {
		case 15:
			data["ReminderDesc"] = "15 minutes before"
		case 30:
			data["ReminderDesc"] = "30 minutes before"
		case 60:
			data["ReminderDesc"] = "1 hour before"
		case 1440:
			data["ReminderDesc"] = "1 day before"
		default:
			data["ReminderDesc"] = fmt.Sprintf("%d minutes before", *event.ReminderMinutes)
		}
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

// LicenseSettingsPartial renders the license/subscription settings card content.
// TunnelSettingsPartial renders the tunnel settings card content.
func (h *TemplateHandler) TunnelSettingsPartial(w http.ResponseWriter, r *http.Request) {
	hasTunnel := h.licenseClient.HasFeature("tunnel")
	status := h.tunnelManager.Status()
	tunnelSettings, _ := h.settingsStore.GetTunnelSettings()
	data := map[string]any{
		"HasTunnel":     hasTunnel,
		"TunnelState":   string(status.State),
		"TunnelSub":     status.Subdomain,
		"TunnelError":   status.Error,
		"TunnelEnabled": tunnelSettings["tunnel_enabled"] == "true",
		"TunnelToken":   tunnelSettings["tunnel_token"],
	}
	h.renderPartial(w, "tunnel-settings-form", data)
}

// TunnelSettingsUpdate handles saving tunnel settings.
func (h *TemplateHandler) TunnelSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderToast(w, "error", "Invalid form data")
		return
	}

	token := strings.TrimSpace(r.FormValue("tunnel_token"))
	enabled := r.FormValue("tunnel_enabled") == "true"

	h.settingsStore.Set("tunnel_token", token)
	h.settingsStore.Set("tunnel_enabled", fmt.Sprintf("%t", enabled))

	h.tunnelManager.UpdateConfig(tunnel.Config{
		Token:   token,
		Enabled: enabled,
	})

	status := h.tunnelManager.Status()
	data := map[string]any{
		"HasTunnel":     h.licenseClient.HasFeature("tunnel"),
		"TunnelState":   string(status.State),
		"TunnelSub":     status.Subdomain,
		"TunnelError":   status.Error,
		"TunnelEnabled": enabled,
		"TunnelToken":   token,
	}

	w.Header().Set("HX-Trigger", `{"showToast": "Tunnel settings updated"}`)
	h.renderPartial(w, "tunnel-settings-form", data)
}

// TunnelStatusPartial returns just the tunnel status badge for polling.
func (h *TemplateHandler) TunnelStatusPartial(w http.ResponseWriter, r *http.Request) {
	status := h.tunnelManager.Status()
	data := map[string]any{
		"TunnelState": string(status.State),
		"TunnelSub":   status.Subdomain,
		"TunnelError": status.Error,
	}
	h.renderPartial(w, "tunnel-status-badge", data)
}

func (h *TemplateHandler) LicenseSettingsPartial(w http.ResponseWriter, r *http.Request) {
	status := h.licenseClient.Status()
	data := map[string]any{
		"IsFreeTier": h.licenseClient.IsFreeTier(),
		"Plan":       status.Plan,
		"Valid":      status.Valid,
		"Features":   status.Features,
		"ExpiresAt":  status.ExpiresAt,
		"Warning":    status.Warning,
		"Offline":    status.Offline,
	}
	h.renderPartial(w, "license-settings-form", data)
}

// LicenseKeyUpdate handles updating the license key from settings.
func (h *TemplateHandler) LicenseKeyUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderToast(w, "error", "Invalid form data")
		return
	}

	key := strings.TrimSpace(r.FormValue("license_key"))
	h.licenseClient.SetKey(key)
	h.settingsStore.Set("license_key", key)

	status := h.licenseClient.Status()
	data := map[string]any{
		"IsFreeTier": h.licenseClient.IsFreeTier(),
		"Plan":       status.Plan,
		"Valid":      status.Valid,
		"Features":   status.Features,
		"ExpiresAt":  status.ExpiresAt,
		"Warning":    status.Warning,
		"Offline":    status.Offline,
	}

	w.Header().Set("HX-Trigger", `{"showToast": "License key updated"}`)
	h.renderPartial(w, "license-settings-form", data)
}

// S3SettingsPartial renders the S3 storage settings card content.
func (h *TemplateHandler) S3SettingsPartial(w http.ResponseWriter, r *http.Request) {
	s3Settings, _ := h.settingsStore.GetS3Settings()
	configured := s3Settings["backup_s3_bucket"] != "" && s3Settings["backup_s3_access_key"] != "" && s3Settings["backup_s3_secret_key"] != ""
	data := map[string]any{
		"Endpoint":   s3Settings["backup_s3_endpoint"],
		"Bucket":     s3Settings["backup_s3_bucket"],
		"Region":     s3Settings["backup_s3_region"],
		"AccessKey":  s3Settings["backup_s3_access_key"],
		"SecretKey":  s3Settings["backup_s3_secret_key"],
		"Configured": configured,
	}
	h.renderPartial(w, "s3-settings-form", data)
}

// S3SettingsUpdate handles saving S3 storage settings.
func (h *TemplateHandler) S3SettingsUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderToast(w, "error", "Invalid form data")
		return
	}

	endpoint := strings.TrimSpace(r.FormValue("backup_s3_endpoint"))
	bucket := strings.TrimSpace(r.FormValue("backup_s3_bucket"))
	region := strings.TrimSpace(r.FormValue("backup_s3_region"))
	accessKey := strings.TrimSpace(r.FormValue("backup_s3_access_key"))
	secretKey := strings.TrimSpace(r.FormValue("backup_s3_secret_key"))

	h.settingsStore.Set("backup_s3_endpoint", endpoint)
	h.settingsStore.Set("backup_s3_bucket", bucket)
	h.settingsStore.Set("backup_s3_region", region)
	h.settingsStore.Set("backup_s3_access_key", accessKey)
	h.settingsStore.Set("backup_s3_secret_key", secretKey)

	if h.backupManager != nil {
		h.backupManager.UpdateS3Config(backup.S3Config{
			Endpoint:  endpoint,
			Bucket:    bucket,
			Region:    region,
			AccessKey: accessKey,
			SecretKey: secretKey,
		})
	}

	h.broadcast(websocket.NewMessage("settings", "updated", 0, nil))

	w.Header().Set("HX-Trigger", `{"showToast": "S3 storage settings updated"}`)
	h.S3SettingsPartial(w, r)
}

func (h *TemplateHandler) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(w, name, data); err != nil {
		h.logger.Error("template error", "error", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (h *TemplateHandler) renderPartial(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(w, name, data); err != nil {
		h.logger.Error("template render failed", "template", name, "error", err)
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

// BackupSettingsPartial renders the backup settings card content.
func (h *TemplateHandler) BackupSettingsPartial(w http.ResponseWriter, r *http.Request) {
	hasBackup := h.licenseClient.HasFeature("backup")
	status := h.backupManager.Status()
	backupSettings, _ := h.settingsStore.GetBackupSettings()

	householdID := auth.HouseholdID(r.Context())
	history, _ := h.backupStore.List(householdID, 10)
	totalSize, _ := h.backupStore.TotalSizeByHousehold(householdID)

	passphraseSet := backupSettings["backup_passphrase_hash"] != ""

	data := map[string]any{
		"HasBackup":        hasBackup,
		"BackupState":      string(status.State),
		"BackupError":      status.Error,
		"BackupInProgress": status.InProgress,
		"BackupEnabled":    backupSettings["backup_enabled"] == "true",
		"ScheduleHour":     backupSettings["backup_schedule_hour"],
		"RetentionDays":    backupSettings["backup_retention_days"],
		"PassphraseSet":    passphraseSet,
		"HasCachedKey":     h.backupManager.HasCachedKey(householdID),
		"History":          history,
		"TotalSize":        totalSize,
	}
	h.renderPartial(w, "backup-settings-form", data)
}

// BackupSettingsUpdate handles saving backup schedule settings.
func (h *TemplateHandler) BackupSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderToast(w, "error", "Invalid form data")
		return
	}

	enabled := r.FormValue("backup_enabled") == "true"
	scheduleHour := r.FormValue("backup_schedule_hour")
	retentionDays := r.FormValue("backup_retention_days")

	h.settingsStore.Set("backup_enabled", fmt.Sprintf("%t", enabled))
	if scheduleHour != "" {
		h.settingsStore.Set("backup_schedule_hour", scheduleHour)
	}
	if retentionDays != "" {
		h.settingsStore.Set("backup_retention_days", retentionDays)
	}

	w.Header().Set("HX-Trigger", `{"showToast": "Backup settings updated"}`)
	h.BackupSettingsPartial(w, r)
}

// BackupPassphraseUpdate handles setting or changing the backup passphrase.
func (h *TemplateHandler) BackupPassphraseUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderToast(w, "error", "Invalid form data")
		return
	}

	passphrase := r.FormValue("passphrase")
	if len(passphrase) < 8 {
		h.renderToast(w, "error", "Passphrase must be at least 8 characters")
		return
	}

	// Generate salt
	salt, err := backup.GenerateSalt()
	if err != nil {
		h.renderToast(w, "error", "Failed to generate salt")
		return
	}

	// Hash passphrase with bcrypt for verification
	hash, err := bcrypt.GenerateFromPassword([]byte(passphrase), bcrypt.DefaultCost)
	if err != nil {
		h.renderToast(w, "error", "Failed to hash passphrase")
		return
	}

	// Store salt as hex and hash
	saltHex := fmt.Sprintf("%x", salt)
	h.settingsStore.Set("backup_passphrase_salt", saltHex)
	h.settingsStore.Set("backup_passphrase_hash", string(hash))

	// Cache key for scheduled backups
	householdID := auth.HouseholdID(r.Context())
	h.backupManager.CacheKey(householdID, passphrase, salt)

	w.Header().Set("HX-Trigger", `{"showToast": "Backup passphrase saved"}`)
	h.BackupSettingsPartial(w, r)
}

// BackupNow triggers an immediate backup.
func (h *TemplateHandler) BackupNow(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderToast(w, "error", "Invalid form data")
		return
	}

	passphrase := r.FormValue("passphrase")
	if passphrase == "" {
		h.renderToast(w, "error", "Passphrase required")
		return
	}

	// Verify passphrase against stored hash
	backupSettings, _ := h.settingsStore.GetBackupSettings()
	storedHash := backupSettings["backup_passphrase_hash"]
	if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(passphrase)); err != nil {
		h.renderToast(w, "error", "Incorrect passphrase")
		return
	}

	householdID := auth.HouseholdID(r.Context())

	go func() {
		if _, err := h.backupManager.RunNow(r.Context(), householdID, passphrase); err != nil {
			h.logger.Error("backup now failed", "error", err)
		}
	}()

	h.renderToast(w, "success", "Backup started")
}

// BackupHistoryPartial renders the backup history list.
func (h *TemplateHandler) BackupHistoryPartial(w http.ResponseWriter, r *http.Request) {
	householdID := auth.HouseholdID(r.Context())
	history, _ := h.backupStore.List(householdID, 20)
	data := map[string]any{
		"History": history,
	}
	h.renderPartial(w, "backup-history-list", data)
}

// BackupStatusPartial returns the backup status badge for polling.
func (h *TemplateHandler) BackupStatusPartial(w http.ResponseWriter, r *http.Request) {
	status := h.backupManager.Status()
	data := map[string]any{
		"BackupState":      string(status.State),
		"BackupError":      status.Error,
		"BackupInProgress": status.InProgress,
	}
	h.renderPartial(w, "backup-status-badge", data)
}

// BackupRestore triggers a restore from a backup.
func (h *TemplateHandler) BackupRestore(w http.ResponseWriter, r *http.Request) {
	if !auth.IsAdmin(r.Context()) {
		h.renderToast(w, "error", "Admin access required")
		return
	}

	if err := r.ParseForm(); err != nil {
		h.renderToast(w, "error", "Invalid form data")
		return
	}

	idStr := r.PathValue("id")
	backupID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.renderToast(w, "error", "Invalid backup ID")
		return
	}

	passphrase := r.FormValue("passphrase")
	if passphrase == "" {
		h.renderToast(w, "error", "Passphrase required")
		return
	}

	// Verify passphrase
	backupSettings, _ := h.settingsStore.GetBackupSettings()
	storedHash := backupSettings["backup_passphrase_hash"]
	if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(passphrase)); err != nil {
		h.renderToast(w, "error", "Incorrect passphrase")
		return
	}

	householdID := auth.HouseholdID(r.Context())

	if err := h.backupManager.Restore(r.Context(), backupID, householdID, passphrase); err != nil {
		h.renderToast(w, "error", fmt.Sprintf("Restore failed: %v", err))
		return
	}
}

// BackupDownload streams an encrypted backup file.
func (h *TemplateHandler) BackupDownload(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	backupID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid backup ID", http.StatusBadRequest)
		return
	}

	householdID := auth.HouseholdID(r.Context())

	record, err := h.backupStore.GetByID(backupID, householdID)
	if err != nil || record == nil {
		http.Error(w, "Backup not found", http.StatusNotFound)
		return
	}

	body, size, err := h.backupManager.Download(r.Context(), backupID, householdID)
	if err != nil {
		http.Error(w, "Download failed", http.StatusInternalServerError)
		return
	}
	defer body.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", record.Filename))
	if size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	}
	io.Copy(w, body)
}

// PushSettingsPartial renders the push notification settings card content.
func (h *TemplateHandler) PushSettingsPartial(w http.ResponseWriter, r *http.Request) {
	hasPush := h.licenseClient.HasFeature("push_notifications")
	userID := auth.UserID(r.Context())
	householdID := auth.HouseholdID(r.Context())

	data := map[string]any{
		"HasPush": hasPush,
	}

	if hasPush && h.pushStore != nil {
		subs, _ := h.pushStore.ListByUser(userID, householdID)
		prefs, _ := h.pushStore.GetPreferences(userID, householdID)

		// Build preference map with defaults
		prefMap := map[string]bool{
			model.NotifTypeCalendarReminder: true,
			model.NotifTypeChoreDue:         true,
			model.NotifTypeGroceryAdded:     true,
		}
		for _, p := range prefs {
			prefMap[p.NotificationType] = p.Enabled
		}

		var vapidKey string
		if h.pushService != nil {
			vapidKey = h.pushService.VAPIDPublicKey()
		}

		data["Subscriptions"] = subs
		data["SubscriptionCount"] = len(subs)
		data["CalendarEnabled"] = prefMap[model.NotifTypeCalendarReminder]
		data["ChoreEnabled"] = prefMap[model.NotifTypeChoreDue]
		data["GroceryEnabled"] = prefMap[model.NotifTypeGroceryAdded]
		data["VAPIDKey"] = vapidKey
	}

	// Add VAPID key info from DB for display
	if vapidSettings, err := h.settingsStore.GetVAPIDSettings(); err == nil {
		data["VAPIDPublicKeyFull"] = vapidSettings["vapid_public_key"]
		data["VAPIDKeysConfigured"] = vapidSettings["vapid_public_key"] != "" && vapidSettings["vapid_private_key"] != ""
	}

	h.renderPartial(w, "push-settings-form", data)
}

// PushPreferencesUpdate handles saving push notification preferences.
func (h *TemplateHandler) PushPreferencesUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderToast(w, "error", "Invalid form data")
		return
	}

	userID := auth.UserID(r.Context())
	householdID := auth.HouseholdID(r.Context())

	calendarEnabled := r.FormValue("calendar_enabled") == "true"
	choreEnabled := r.FormValue("chore_enabled") == "true"
	groceryEnabled := r.FormValue("grocery_enabled") == "true"

	h.pushStore.SetPreference(userID, householdID, model.NotifTypeCalendarReminder, calendarEnabled)
	h.pushStore.SetPreference(userID, householdID, model.NotifTypeChoreDue, choreEnabled)
	h.pushStore.SetPreference(userID, householdID, model.NotifTypeGroceryAdded, groceryEnabled)

	w.Header().Set("HX-Trigger", `{"showToast": "Notification preferences updated"}`)
	h.PushSettingsPartial(w, r)
}

// PushDevicesList renders the push subscription device list.
func (h *TemplateHandler) PushDevicesList(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())
	householdID := auth.HouseholdID(r.Context())

	subs, _ := h.pushStore.ListByUser(userID, householdID)
	h.renderPartial(w, "push-devices-list", map[string]any{
		"Subscriptions": subs,
	})
}

// PushDeviceDelete removes a push subscription.
func (h *TemplateHandler) PushDeviceDelete(w http.ResponseWriter, r *http.Request) {
	householdID := auth.HouseholdID(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	h.pushStore.DeleteSubscription(id, householdID)
	w.Header().Set("HX-Trigger", `{"showToast": "Device removed"}`)
	h.PushSettingsPartial(w, r)
}
