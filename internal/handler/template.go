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
		avatarEmoji = "😀"
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
