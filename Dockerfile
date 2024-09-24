# Используем официальный образ MongoDB
FROM mongo:latest

# Задаем переменные окружения (не обязательно)
ENV MONGO_INITDB_ROOT_USERNAME=FunnyRockfish
ENV MONGO_INITDB_ROOT_PASSWORD=homework3

# Экспонируем стандартный порт MongoDB
EXPOSE 27017

# Команда запуска MongoDB
CMD ["mongod"]
