FROM golang:1.22-alpine AS build
WORKDIR /src

RUN apk add --no-cache git ca-certificates && update-ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/admin-api ./admin

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && update-ca-certificates
WORKDIR /app
COPY --from=build /out/admin-api ./admin-api
COPY admin/etc ./etc
EXPOSE 9005
CMD ["./admin-api", "-f", "etc/admin-api.yaml"]
