---
to: {{ name | snake }}.py
desc: Create a new endpoint handler
---
from flask import jsonify


def get_{{ name | snake }}(city: str) -> dict:
    """Fetch {{ name | snake }} data for a city."""
    return {
        "city": city,
        "{{ name | snake }}": "placeholder",
    }


def {{ name | snake }}_handler(city: str):
    """Handle /{{ name | kebab }}/<city> requests."""
    data = get_{{ name | snake }}(city)
    return jsonify(data)
