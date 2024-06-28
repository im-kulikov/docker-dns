FROM golang:alpine AS builder

LABEL stage=gobuilder

ENV CGO_ENABLED=0

RUN apk update --no-cache && apk add --no-cache tzdata

WORKDIR /build

ADD go.mod .
ADD go.sum .
RUN go mod download
COPY . .
RUN go build -ldflags="-s -w" -o /app/docker-dns /build/cmd/dns
RUN go build -ldflags="-s -w" -o /app/docker-bgp /build/cmd/bgp


FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /usr/share/zoneinfo/Europe/Moscow /usr/share/zoneinfo/Europe/Moscow


WORKDIR /app
COPY --from=builder /app/docker-bgp /app/docker-bgp
COPY --from=builder /app/docker-dns /app/docker-dns

CMD ["./docker-dns"]
