package handler

import (
	"html/template"
	"log/slog"
	"net/http"
	"time"
)

type MarketingHandler struct {
	templates map[string]*template.Template
	baseURL   string
	logger    *slog.Logger
}

func NewMarketingHandler(
	tmpl map[string]*template.Template,
	baseURL string,
	logger *slog.Logger,
) *MarketingHandler {
	return &MarketingHandler{
		templates: tmpl,
		baseURL:   baseURL,
		logger:    logger,
	}
}

// LandingPage renders the homepage.
func (h *MarketingHandler) LandingPage(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"BaseURL":   h.baseURL,
		"Year":      time.Now().Year(),
		"ActiveNav": "home",
	}
	h.render(w, "index.html", data)
}

// FeaturesPage renders the features page.
func (h *MarketingHandler) FeaturesPage(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"BaseURL":   h.baseURL,
		"Year":      time.Now().Year(),
		"ActiveNav": "features",
	}
	h.render(w, "features.html", data)
}

func (h *MarketingHandler) render(w http.ResponseWriter, name string, data any) {
	tmpl, ok := h.templates[name]
	if !ok {
		h.logger.Error("template not found", "name", name)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		h.logger.Error("template render", "error", err)
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}
