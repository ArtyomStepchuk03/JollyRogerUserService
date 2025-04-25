.PHONY: build test lint run clean docker-build docker-run docker-compose docker-down proto

APP_NAME=user-service
GOPATH:=$(shell go env GOPATH)

# Сборка приложения
build:
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o $(APP_NAME) ./cmd/main.go

# Запуск тестов
test:
	go test -v ./internal/...

# Запуск интеграционных тестов
test-integration:
	go test -v ./tests/integration/...

# Запуск всех тестов с покрытием
test-coverage:
	go test -v -coverprofile=coverage.out ./internal/... ./test/... ./pkg/...
	go tool cover -html=coverage.out -o coverage.html

# Линтинг кода
lint:
	golangci-lint run ./...

# Локальный запуск
run:
	go run ./cmd/main.go

# Очистка бинарных файлов
clean:
	rm -f $(APP_NAME)
	rm -f coverage.out
	rm -f coverage.html

# Сборка Docker образа
docker-build:
	docker build -t $(APP_NAME):latest .

# Запуск Docker контейнера
docker-run:
	docker run -p 50051:50051 --name $(APP_NAME) $(APP_NAME):latest

# Запуск через Docker Compose
docker-compose:
	docker-compose up -d

# Остановка Docker Compose
docker-down:
	docker-compose down

# Генерация кода из proto файлов
proto:
	protoc --go_out=. --go_opt=paths=source_relative \
	--go-grpc_out=. --go-grpc_opt=paths=source_relative \
	pkg/proto/user/user.proto

# Миграции БД
migrate-up:
	migrate -path migrations -database "postgres://postgres:postgres@localhost:5432/jolly_roger_user_db?sslmode=disable" up

migrate-down:
	migrate -path migrations -database "postgres://postgres:postgres@localhost:5432/jolly_roger_user_db?sslmode=disable" down

# Помощь
help:
	@echo "Доступные команды:"
	@echo "  make build              - Сборка приложения"
	@echo "  make test               - Запуск модульных тестов"
	@echo "  make test-integration   - Запуск интеграционных тестов"
	@echo "  make test-coverage      - Запуск тестов с отчетом о покрытии"
	@echo "  make lint               - Линтинг кода"
	@echo "  make run                - Локальный запуск"
	@echo "  make clean              - Очистка бинарных файлов"
	@echo "  make docker-build       - Сборка Docker образа"
	@echo "  make docker-run         - Запуск Docker контейнера"
	@echo "  make docker-compose     - Запуск через Docker Compose"
	@echo "  make docker-down        - Остановка Docker Compose"
	@echo "  make proto              - Генерация кода из proto файлов"
	@echo "  make migrate-up         - Выполнение миграций БД вверх"
	@echo "  make migrate-down       - Выполнение миграций БД вниз"