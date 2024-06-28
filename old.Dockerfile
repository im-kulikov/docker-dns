FROM golang:1.21.4 as builder

COPY . /app

WORKDIR /app

ENV CGO_ENABLED=0

RUN go mod tidy
RUN go build -o docker-dns /app/cmd/dns

FROM alpine:latest as runner

COPY --from=builder /app/docker-dns /bin/docker-dns

CMD docker-dns