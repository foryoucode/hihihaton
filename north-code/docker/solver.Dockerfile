# syntax=docker/dockerfile:1
FROM python:3.12-slim-bookworm
WORKDIR /app
ENV PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1 SOLVER_HOST=0.0.0.0 SOLVER_PORT=8091
COPY solver/requirements.txt ./requirements.txt
RUN --mount=type=secret,id=ca_bundle \
    if [ -f /run/secrets/ca_bundle ]; then export PIP_CERT=/run/secrets/ca_bundle; fi; \
    pip install --no-cache-dir -r requirements.txt
COPY --chown=10001:10001 solver/server.py solver/optimizer.py ./
USER 10001:10001
EXPOSE 8091
CMD ["python", "server.py"]
