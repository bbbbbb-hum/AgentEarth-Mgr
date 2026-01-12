ARG DOCKER_REP_PATH=
FROM ${DOCKER_REP_PATH}golang:1.22 AS build
WORKDIR /src

RUN apt-get update \
  && apt-get install -y --no-install-recommends git ca-certificates \
  && update-ca-certificates \
  && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/admin-api ./admin

FROM ${DOCKER_REP_PATH}alpine:latest
RUN apk add --no-cache ca-certificates && update-ca-certificates
WORKDIR /app
COPY --from=build /out/admin-api ./admin-api
COPY admin/etc ./etc
EXPOSE 9005
CMD ["./admin-api", "-f", "etc/admin-api.yaml"]
