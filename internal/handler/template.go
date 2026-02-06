package handler

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

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

// CalendarPartial renders the calendar placeholder for HTMX swap.
func (h *TemplateHandler) CalendarPartial(w http.ResponseWriter, r *http.Request) {
	h.renderPartial(w, "calendar-content", nil)
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
