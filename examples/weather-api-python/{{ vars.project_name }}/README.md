# {{ vars.project_name }}

{{ vars.description }}

## Getting Started

1. Install dependencies:

   ```bash
   pip install -r requirements.txt
   ```

2. Run the server:

   ```bash
   python app.py
   ```

3. Try it out:

   ```bash
   curl http://localhost:5000/weather/London
   ```

{% if vars.api_provider != "Mock" %}
## Configuration

Set your {{ vars.api_provider }} API key in `app.py` before running.
{% endif %}
{% if vars.use_docker %}
## Docker

```bash
docker build -t {{ vars.project_name | kebab }} .
docker run -p 5000:5000 {{ vars.project_name | kebab }}
```
{% endif %}
