package app

import "net/http"

func RegisterRoutes(mux *http.ServeMux) {
	// routes
	mux.HandleFunc("/health", healthHandler)
}
