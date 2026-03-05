---
to: main.go
inject: true
after: "// ROUTES"
desc: Register the new endpoint route
notes: "The new /{{ name | kebab }} endpoint is ready. Restart the server to use it."
---
	mux.HandleFunc("GET /{{ name | kebab }}/{city}", {{ name | camel }}Handler)
