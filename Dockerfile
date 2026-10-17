# Builds the server with the dashboard embedded (see Makefile `build`).

FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
# vite.config.ts writes to ../internal/webui/dist/app.
RUN npm run build

FROM golang:1.25 AS server
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY --from=web /src/internal/webui/dist/app internal/webui/dist/app
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# distroless/static ships CA certificates (for Bungie and Postgres TLS) and runs as nonroot.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=server /out/server /server
EXPOSE 8080
ENV PORT=8080 LOG_FORMAT=json
ENTRYPOINT ["/server"]
