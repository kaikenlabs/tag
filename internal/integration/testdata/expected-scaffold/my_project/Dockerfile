
FROM python:3.11-slim

WORKDIR /app
COPY . .
RUN pip install -e .

EXPOSE 8080.0

CMD ["{{ vars.project_name|lower|replace(' ', '_') }}"]

