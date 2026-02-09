package app

func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", healthHandler)
		mux.HandleFunc("/users", usersHandler)
// end routes
}
