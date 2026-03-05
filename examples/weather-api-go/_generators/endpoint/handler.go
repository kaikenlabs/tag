---
to: {{ name | snake }}.go
desc: Create a new endpoint handler
---
package main

import (
	"encoding/json"
	"net/http"
)

// {{ name | pascal }}Data holds {{ name | snake }} information.
type {{ name | pascal }}Data struct {
	City string `json:"city"`
	{{ name | pascal }} string `json:"{{ name | snake }}"`
}

func get{{ name | pascal }}(city string) {{ name | pascal }}Data {
	return {{ name | pascal }}Data{
		City: city,
		{{ name | pascal }}: "placeholder",
	}
}

func {{ name | camel }}Handler(w http.ResponseWriter, r *http.Request) {
	city := r.PathValue("city")
	if city == "" {
		http.Error(w, "city parameter required", http.StatusBadRequest)
		return
	}
	data := get{{ name | pascal }}(city)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
