ARG DOCKER_REP_PATH=
# 降级到 1.21-alpine，这是最经典的版本，加速器很大几率有缓存
FROM ${DOCKER_REP_PATH}golang:1.21-alpine AS build
WORKDIR /src

RUN apk add --no-cache git ca-certificates && update-ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/admin-api ./admin

ARG DOCKER_REP_PATH=
# 运行环境改为 latest 或更旧的 3.19
FROM ${DOCKER_REP_PATH}alpine:latest
RUN apk add --no-cache ca-certificates && update-ca-certificates
WORKDIR /app
COPY --from=build /out/admin-api ./admin-api
COPY admin/etc ./etc
EXPOSE 9005
CMD ["./admin-api", "-f", "etc/admin-api.yaml"]
