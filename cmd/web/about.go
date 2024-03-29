package web

import (
	"log"
	"net/http"

	"plugtalk/internal/shared"
)

func AboutHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	aboutPage := About(shared.Themes)
	err = aboutPage.Render(r.Context(), w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Printf("Error rendering in HelloWebHandler: %v", err)
		return
	}
}
