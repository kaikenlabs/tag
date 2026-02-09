{% if vars.use_docker %}
FROM python:3.11-slim

WORKDIR /app
COPY . .
RUN pip install -e .

EXPOSE {{ vars.port }}

CMD ["{{ vars.project_slug }}"]
{% endif %}
