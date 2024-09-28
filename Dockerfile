FROM golang:1.21-alpine AS build-stage

WORKDIR /app

RUN apk update && apk add --no-cache git bash build-base

ENV GOMODCACHE /go/pkg/mod
ENV GOCACHE /go-cache

COPY . .

RUN go mod download

RUN CGO_ENABLED=0 go build -o /app/proxyserver ./cmd/main.go  # Проверьте, что путь к main.go правильный

RUN chmod 644 /app/cmd/certs/ca.crt /app/cmd/certs/ca.key

FROM gcr.io/distroless/base-debian11 AS build-release-stage


WORKDIR /app

COPY --from=build-stage /app/proxyserver /app/proxyserver
COPY --from=build-stage /app/cmd/certs /app/certs


EXPOSE 8000 8080

CMD ["/app/proxyserver"]
