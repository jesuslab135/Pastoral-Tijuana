# One image carries the three binaries (api, worker, setup); compose picks
# the entrypoint per service, so every deploy ships one backend artifact.
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/setup ./cmd/setup

# distroless/static: CA certs for SMTP TLS, /etc/passwd for nonroot, no shell.
# The binaries embed tzdata themselves (time/tzdata), so PARISH_TZ resolves.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/ /app/
ENTRYPOINT ["/app/api"]
