# Stage 1: Build React + TypeScript UI
FROM node:22-alpine AS ui-builder
WORKDIR /src/web
COPY web/package.json web/package-lock.json* ./
RUN npm ci || npm install
COPY web/ ./
RUN npm run build

# Stage 2: Build Go binary with //go:embed UI
FROM golang:1.26-alpine AS go-builder
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum* ./
RUN go mod download || true
COPY . .
COPY --from=ui-builder /src/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/zetnix ./cmd/zetnix

# Stage 3: Minimal production runtime (~25MB)
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 zetnix
USER zetnix
WORKDIR /app
COPY --from=go-builder /out/zetnix /usr/local/bin/zetnix
EXPOSE 8080
ENTRYPOINT ["zetnix"]
CMD ["--addr", ":8080"]
