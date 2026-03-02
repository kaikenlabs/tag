package app

import "net/http"

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
}

func usersHandler(w http.ResponseWriter, r *http.Request) {
	// handle users
}
