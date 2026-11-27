# --- Сборка ---
FROM golang:1.27-alpine AS build

WORKDIR /src

COPY backend/go.* ./
RUN go mod download


COPY . .
RUN CGO_ENABLED=0 go build -o /gkh .

# --- Запуск ---
FROM alpine:3.20
WORKDIR /app

COPY --from=build /gkh /app/gkh
COPY backend/schema.sql /app/schema.sql


# данные вне контейнера: база и чеки
ENV GKH_DB=/app/data/gkh.db GKH_UPLOADS=/app/uploads

EXPOSE 8091
ENTRYPOINT ["/app/gkh"]
