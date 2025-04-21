FROM golang:1.21-alpine AS builder

WORKDIR /app

# Копируем файлы go.mod и go.sum
COPY go.mod go.sum ./

# Загружаем зависимости
RUN go mod download

# Копируем весь исходный код
COPY . .

# Компилируем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o user-service ./cmd/server

# Используем минимальный образ для запуска
FROM alpine:3.17

# Установка необходимых пакетов
RUN apk --no-cache add ca-certificates tzdata && \
    mkdir -p /app/config

WORKDIR /app

# Копируем скомпилированное приложение
COPY --from=builder /app/user-service .
COPY --from=builder /app/config/config.yaml ./config/

# Устанавливаем переменные окружения
ENV TZ=UTC \
    LOG_LEVEL=info

# Открываем порт, на котором работает приложение
EXPOSE 50051

# Команда запуска приложения
CMD ["./user-service"]