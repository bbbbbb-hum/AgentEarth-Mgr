# 1. 构建阶段：使用阿里云官方公共镜像站，确保 100% 存在且稳定
FROM registry.cn-hangzhou.aliyuncs.com/library/golang:1.22-alpine AS build
WORKDIR /src

RUN apk add --no-cache git ca-certificates && update-ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/admin-api ./admin

# 2. 运行阶段：回归变量引用，尊重流水线全局配置
ARG DOCKER_REP_PATH=
FROM ${DOCKER_REP_PATH}alpine:3.20
RUN apk add --no-cache ca-certificates && update-ca-certificates
WORKDIR /app
COPY --from=build /out/admin-api ./admin-api
COPY admin/etc ./etc
EXPOSE 9005
CMD ["./admin-api", "-f", "etc/admin-api.yaml"]
