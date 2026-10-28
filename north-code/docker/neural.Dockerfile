# syntax=docker/dockerfile:1

FROM python:3.12-slim-bookworm
WORKDIR /app
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    OPENBLAS_NUM_THREADS=1 \
    NEURAL_HOST=0.0.0.0 \
    NEURAL_PORT=8091
COPY engine/requirements-inference.txt ./requirements.txt
RUN --mount=type=secret,id=ca_bundle \
    if [ -f /run/secrets/ca_bundle ]; then export PIP_CERT=/run/secrets/ca_bundle; fi; \
    pip install --no-cache-dir -r requirements.txt
COPY --chown=10001:10001 engine/server.py engine/neural.py engine/common.py ./
COPY --chown=10001:10001 engine/models/ranker.npz /app/models/ranker.npz
USER 10001:10001
EXPOSE 8091
CMD ["python", "server.py"]
