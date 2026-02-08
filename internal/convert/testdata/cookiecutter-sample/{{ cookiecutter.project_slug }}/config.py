"""Configuration with Jinja2-specific syntax for testing."""

# This file has Jinja2 syntax that differs from Gonja

# Filter with parentheses (Jinja2 style)
DEFAULT_NAME = "{{ cookiecutter.author | default('Anonymous') }}"

# Dict iteration (Jinja2 style)
{% for key, value in settings.items() %}
{{ key }} = {{ value }}
{% endfor %}

# Standard filter (compatible)
PROJECT_SLUG = "{{ cookiecutter.project_slug | lower }}"
