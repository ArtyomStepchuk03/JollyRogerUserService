# JollyRogerUserService

Сервис управления пользователями для проекта JollyRoger - платформы для поиска и создания команд для совместных мероприятий.

## Описание

User Service — это микросервис, отвечающий за управление пользовательскими данными:
- Профили пользователей
- Рейтинги пользователей
- Предпочтения пользователей (теги)
- Геолокации пользователей
- Настройки уведомлений

Сервис реализует API через gRPC и использует PostgreSQL для хранения данных и Redis для кэширования.

## Технологии

- **Язык**: Go
- **API**: gRPC
- **Хранение данных**: PostgreSQL, GORM
- **Кэширование**: Redis
- **Контейнеризация**: Docker, Docker Compose
- **Логирование**: Zap

## Структура проекта

```
├── cmd/                       # Точка входа в приложение
│   └── main.go
├── config/                    # Конфигурация приложения
│   ├── config.go
│   └── config.yaml
├── internal/                  # Внутренний код приложения
│   ├── delivery/              # Обработчики запросов
│   │   └── grpc/              # gRPC обработчики
│   ├── models/                # Модели данных
│   ├── repository/            # Репозитории для работы с хранилищами
│   │   ├── postgres/
│   │   └── redis/
│   └── service/               # Бизнес-логика
├── pkg/                       # Общие пакеты
│   ├── database/              # Подключение к БД
│   ├── logger/                # Настройка логгера
│   └── proto/                 # Протофайлы
│       └── user/
├── tests/                     # Тесты
│   └── integration/
├── Dockerfile
├── docker-compose.yml
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## Конфигурация

Сервис настраивается через файл `.env`:

```
GRPC_PORT=

DB_CONTAINER_NAME=
CACHE_CONTAINER_NAME=
SERVICE_CONTAINER_NAME=
DB_HOST=
DB_PORT=
DB_NAME=
DB_USER=
DB_PASSWORD=
DB_SSLMODE=

REDIS_HOST=
REDIS_PORT=
REDIS_PASSWORD=
REDIS_DB=
```

## Установка и запуск

### Локальный запуск

1. Установите Go (версия 1.21 или выше)
2. Клонируйте репозиторий
3. Создайте файл `.env` с необходимыми переменными окружения
4. Запустите PostgreSQL и Redis локально
5. Запустите сервис:

```bash
make run
```

### Запуск через Docker Compose

1. Установите Docker и Docker Compose
2. Запустите сервисы:

```bash
make docker-compose
```

## API

Сервис предоставляет следующие gRPC методы:

- `CreateUser` - создание нового пользователя
- `GetUser` - получение пользователя по ID
- `GetUserByTelegramID` - получение пользователя по Telegram ID
- `UpdateUser` - обновление данных пользователя
- `AddUserPreference` - добавление предпочтения
- `RemoveUserPreference` - удаление предпочтения
- `GetUserPreferences` - получение всех предпочтений пользователя
- `UpdateUserLocation` - обновление местоположения
- `GetUserLocation` - получение местоположения
- `FindNearbyUsers` - поиск пользователей рядом
- `GetUserStats` - получение статистики пользователя
- `UpdateUserRating` - обновление рейтинга пользователя
- `UpdateNotificationSettings` - обновление настроек уведомлений
- `GetNotificationSettings` - получение настроек уведомлений

## Тестирование

### Запуск модульных тестов

```bash
make test
```

### Запуск интеграционных тестов

```bash
make test-integration
```

### Отчет о покрытии тестами

```bash
make test-coverage
```

## CI/CD

TODO: Добавить информацию о CI/CD после настройки.

## Мониторинг

TODO: Добавить информацию о мониторинге после настройки.

## Контрибьюторы

- Степчук Артём Владимирович

## Лицензия

MIT