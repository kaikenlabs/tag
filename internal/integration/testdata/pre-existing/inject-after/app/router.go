package app

func RegisterRoutes(mux *http.ServeMux) {
	// routes
	mux.HandleFunc("/health", healthHandler)
}
