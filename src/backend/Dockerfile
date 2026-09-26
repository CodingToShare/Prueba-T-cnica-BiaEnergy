# syntax=docker/dockerfile:1
# One backend image for every Go command: api (default), migrate, seed and
# healthcheck. Build from the repository root (compose.yaml does this):
#   docker build -f src/backend/Dockerfile -t bia-energy-backend .
# The image carries the migrations and the supplied challenge CSVs, so the
# demo stack is self-contained; nothing else from the repository is copied.

FROM golang:1.27.1-bookworm AS build
WORKDIR /src/backend
COPY src/backend/go.mod src/backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY src/backend/ ./
# Static binaries: the dependencies need no cgo.
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ \
      ./cmd/api ./cmd/migrate ./cmd/seed ./cmd/healthcheck

# Distroless static: no shell or package manager; CA roots and tzdata are
# included (optional HTTPS Ollama). Runs as the unprivileged "nonroot" user.
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/ /app/bin/
# The commands find these by walking up from the working directory.
COPY database/migrations/ /app/database/migrations/
COPY data/input/readings.csv data/input/events.csv /app/data/input/
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT []
CMD ["/app/bin/api"]
