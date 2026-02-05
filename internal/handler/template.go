package handler

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/dukerupert/gamwich/internal/store"
	"golang.org/x/crypto/bcrypt"
)

type TemplateHandler struct {
	store     *store.FamilyMemberStore
	templates *template.Template
}

func NewTemplateHandler(s *store.FamilyMemberStore) *TemplateHandler {
	tmpl := template.Must(template.ParseGlob("web/templates/*.html"))
	return &TemplateHandler{
		store:     s,
		templates: tmpl,
	}
}

func (h *TemplateHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	members, err := h.store.List()
	if err != nil {
		http.Error(w, "failed to load data", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Title":   "Gamwich",
		"Members": members,
	}
	h.render(w, "layout.html", data)
}

func (h *TemplateHandler) FamilyMembers(w http.ResponseWriter, r *http.Request) {
	members, err := h.store.List()
	if err != nil {
		http.Error(w, "failed to load family members", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Title":   "Family Members — Gamwich",
		"Members": members,
	}
	h.render(w, "family_members.html", data)
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
		h.renderPartial(w, "form-error", map[string]string{"Error": "Name is required"})
		return
	}

	if color == "" {
		color = "#3B82F6"
	}
	if !hexColorRegexp.MatchString(color) {
		h.renderPartial(w, "form-error", map[string]string{"Error": "Invalid color format"})
		return
	}

	if avatarEmoji == "" {
		avatarEmoji = "😀"
	}

	exists, err := h.store.NameExists(name, 0)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if exists {
		h.renderPartial(w, "form-error", map[string]string{"Error": "A family member with that name already exists"})
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
		h.renderPartial(w, "form-error", map[string]string{"Error": "Name is required"})
		return
	}

	if !hexColorRegexp.MatchString(color) {
		h.renderPartial(w, "form-error", map[string]string{"Error": "Invalid color format"})
		return
	}

	exists, err := h.store.NameExists(name, id)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if exists {
		h.renderPartial(w, "form-error", map[string]string{"Error": "A family member with that name already exists"})
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
	action := r.FormValue("action")

	if action == "clear" {
		if err := h.store.ClearPIN(id); err != nil {
			http.Error(w, "failed to clear PIN", http.StatusInternalServerError)
			return
		}
		member, _ := h.store.GetByID(id)
		h.renderPartial(w, "pin-status", member)
		return
	}

	if len(pin) != 4 || !isDigits(pin) {
		h.renderPartial(w, "pin-error", map[string]string{"Error": "PIN must be exactly 4 digits"})
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

	member, _ := h.store.GetByID(id)
	h.renderPartial(w, "pin-status", member)
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
