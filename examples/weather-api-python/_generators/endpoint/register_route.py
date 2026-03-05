---
to: app.py
inject: true
after: "# ROUTES"
desc: Register the new endpoint route
notes: "Remember to import the handler in app.py if needed."
---

@app.route("/{{ name | kebab }}/<city>")
def {{ name | snake }}(city: str):
    """Return {{ name | snake }} data as JSON."""
    from {{ name | snake }} import {{ name | snake }}_handler
    return {{ name | snake }}_handler(city)
