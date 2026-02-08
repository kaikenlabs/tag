{% if cookiecutter.use_docker %}
FROM python:3.11-slim

WORKDIR /app
COPY . .
RUN pip install -e .

EXPOSE {{ cookiecutter.port }}

CMD ["{{ cookiecutter.project_slug }}"]
{% endif %}
