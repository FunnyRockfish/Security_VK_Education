FROM golang:1.21-alpine

WORKDIR /hw1

COPY . .

RUN go mod tidy

RUN go build -o hw1-http-proxy-server .

RUN apk add --no-cache curl

EXPOSE 8080

CMD ["./hw1-http-proxy-server"]
