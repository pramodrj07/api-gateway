FROM golang:1.23-alpine
WORKDIR /app
COPY main.go .
COPY config.yaml .

ADD . .
RUN go mod init gateway
RUN go mod tidy
RUN go build -o main main.go
CMD ["./main"]