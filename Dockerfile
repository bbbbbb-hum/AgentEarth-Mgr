ARG DOCKER_REP_PATH=""
FROM ${DOCKER_REP_PATH}ubuntu:22.04

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates \
  && update-ca-certificates \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app
RUN mkdir -p /app/etc

COPY bin/agent-earth-manager /app/agent-earth-manager
COPY admin/etc /app/etc

RUN chmod +x /app/agent-earth-manager

EXPOSE 9005
CMD ["/app/agent-earth-manager", "-f", "/app/etc/admin-api.yaml"]
