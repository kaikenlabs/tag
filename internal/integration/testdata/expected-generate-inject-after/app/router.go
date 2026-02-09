package app

func RegisterRoutes(mux *http.ServeMux) {
	// routes	mux.HandleFunc("/users", usersHandler)

	mux.HandleFunc("/health", healthHandler)
}
