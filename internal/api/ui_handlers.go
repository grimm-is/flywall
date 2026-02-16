// Copyright (C) 2026 Ben Grimm. Licensed under AGPL-3.0 (https://www.gnu.org/licenses/agpl-3.0.txt)

package api

import (
	"net/http"
	"strings"

	"grimm.is/flywall/internal/ui"
)

// handleUIMenu returns the navigation menu schema.
func (s *Server) handleUIMenu(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	data, err := ui.MenuJSON()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(data)
}

// handleUIPages returns all page schemas.
func (s *Server) handleUIPages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	data, err := ui.AllPagesJSON()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(data)
}

// handleUIPage returns a single page schema.
func (s *Server) handleUIPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract page ID from path: /api/ui/page/{id}
	pageID := ui.MenuID(strings.TrimPrefix(r.URL.Path, "/api/ui/page/"))
	if pageID == "" {
		http.Error(w, "Page ID required", http.StatusBadRequest)
		return
	}

	page := ui.GetPage(pageID)
	if page == nil {
		WriteErrorCtx(w, r, http.StatusNotFound, "Page not found")
		return
	}

	WriteJSON(w, http.StatusOK, page)
}
