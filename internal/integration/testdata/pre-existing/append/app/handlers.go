package app

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
}
