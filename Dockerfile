FROM node:20-alpine AS frontend-builder
WORKDIR /app/frontend

COPY frontend/package*.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

FROM golang:1.23-alpine AS backend-builder
WORKDIR /app
RUN apk add --no-cache gcc musl-dev sqlite-dev

COPY backend/go.mod backend/go.sum ./backend/
RUN cd backend && go mod download

COPY backend/ ./backend/
COPY --from=frontend-builder /app/frontend/.output/public ./backend/frontend/dist

WORKDIR /app/backend
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-w -s" -o capacitarr main.go

FROM alpine:latest
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata sqlite-libs

COPY --from=backend-builder /app/backend/capacitarr /app/capacitarr

EXPOSE 8080

ENTRYPOINT ["/app/capacitarr"]
