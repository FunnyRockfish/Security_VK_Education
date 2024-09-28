# 1. Используем официальный образ Go для сборки
FROM golang:1.21-alpine AS build-stage

# Создаем рабочую директорию в контейнере
WORKDIR /app

# Устанавливаем необходимые утилиты
RUN apk update && apk add --no-cache git bash build-base

# Устанавливаем переменные окружения для кэширования модулей Go
ENV GOMODCACHE /go/pkg/mod
ENV GOCACHE /go-cache

# Копируем исходный код и файлы конфигурации из локальной системы
COPY . .

# Загружаем зависимости
RUN go mod download

# Компилируем Go-приложение
RUN CGO_ENABLED=0 go build -o /app/proxyserver ./cmd/main.go  # Проверьте, что путь к main.go правильный

# Устанавливаем права доступа к сертификатам на этапе сборки
RUN chmod 644 /app/cmd/certs/ca.crt /app/cmd/certs/ca.key

# Финальный этап с минимальным образом
FROM gcr.io/distroless/base-debian11 AS build-release-stage

# Устанавливаем рабочую директорию для запуска приложения
WORKDIR /app

# Копируем исполняемый файл и сертификаты из стадии сборки
COPY --from=build-stage /app/proxyserver /app/proxyserver
COPY --from=build-stage /app/cmd/certs /app/certs


# Открываем порты
EXPOSE 8000 8080

# Запуск исполняемого файла
CMD ["/app/proxyserver"]
