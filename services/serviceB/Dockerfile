FROM golang:1.23-alpine
WORKDIR /app
COPY main.go .
RUN go build -o serviceB main.go
CMD ["./serviceB"]