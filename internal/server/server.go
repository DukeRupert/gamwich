package server

import (
	"database/sql"
	"net/http"

	"github.com/dukerupert/gamwich/internal/handler"
	"github.com/dukerupert/gamwich/internal/store"
)

type Server struct {
	db              *sql.DB
	familyMemberH   *handler.FamilyMemberHandler
	templateHandler *handler.TemplateHandler
}

func New(db *sql.DB) *Server {
	familyMemberStore := store.NewFamilyMemberStore(db)

	return &Server{
		db:              db,
		familyMemberH:   handler.NewFamilyMemberHandler(familyMemberStore),
		templateHandler: handler.NewTemplateHandler(familyMemberStore),
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

	// Page routes
	mux.HandleFunc("GET /", s.templateHandler.Dashboard)
	mux.HandleFunc("GET /settings/family", s.templateHandler.FamilyMembers)

	// htmx partial routes
	mux.HandleFunc("GET /partials/family-members", s.templateHandler.FamilyMemberList)
	mux.HandleFunc("GET /partials/family-members/{id}/edit", s.templateHandler.FamilyMemberEditForm)
	mux.HandleFunc("POST /partials/family-members", s.templateHandler.FamilyMemberCreate)
	mux.HandleFunc("PUT /partials/family-members/{id}", s.templateHandler.FamilyMemberUpdate)
	mux.HandleFunc("DELETE /partials/family-members/{id}", s.templateHandler.FamilyMemberDelete)

	// PIN partials
	mux.HandleFunc("GET /partials/family-members/{id}/pin", s.templateHandler.PINSetupForm)
	mux.HandleFunc("POST /partials/family-members/{id}/pin", s.templateHandler.PINSetup)

	// Static files
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	return mux
}
