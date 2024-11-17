FROM golang:1.23-alpine
WORKDIR /app
COPY gateway.go .

ADD . .
# RUN go mod init gateway
RUN go mod tidy
RUN go build -o gateway gateway.go
CMD ["./gateway"]