package app

func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", healthHandler)
	// end routes
}
