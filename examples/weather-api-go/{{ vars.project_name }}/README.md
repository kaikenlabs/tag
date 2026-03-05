# {{ vars.project_name }}

{{ vars.description }}

## Getting Started

1. Build and run:

   ```bash
   go run main.go
   ```

2. Try it out:

   ```bash
   curl http://localhost:8080/weather/London
   ```

{% if vars.api_provider != "Mock" %}
## Configuration

Set your {{ vars.api_provider }} API key in `main.go` before running.
{% endif %}
{% if vars.use_docker %}
## Docker

```bash
docker build -t {{ vars.project_name | kebab }} .
docker run -p 8080:8080 {{ vars.project_name | kebab }}
```
{% endif %}
