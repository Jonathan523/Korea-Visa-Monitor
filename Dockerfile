FROM golang:1.24-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/krvisa ./cmd/krvisa

FROM python:3.13-slim
COPY --from=build /out/krvisa /app/krvisa
RUN useradd --create-home --uid 10001 appuser \
    && mkdir -p /state \
    && chown appuser:appuser /state
USER appuser
ENTRYPOINT ["/app/krvisa"]
