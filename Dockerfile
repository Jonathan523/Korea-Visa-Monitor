FROM golang:1.24-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/krvisa ./cmd/krvisa

FROM alpine:3.22
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/krvisa /app/krvisa
RUN mkdir -p /state && chown 10001:10001 /state
USER 10001:10001
ENTRYPOINT ["/app/krvisa"]
