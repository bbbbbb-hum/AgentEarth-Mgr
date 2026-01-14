ARG DOCKER_REP_PATH=""
FROM ${DOCKER_REP_PATH}ubuntu:22.04

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates \
  && update-ca-certificates \
  && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /opt/xlapps/AEMGRBE/bin \
  && mkdir -p /opt/xltmp/AEMGRBE/tmp \
  && mkdir -p /opt/xldata/AEMGRBE/data

WORKDIR /opt/xlapps/AEMGRBE/bin

COPY bin/agent-earth-mgr-backend /opt/xlapps/AEMGRBE/bin/agent-earth-mgr-backend

RUN chmod +x /opt/xlapps/AEMGRBE/bin/agent-earth-mgr-backend

EXPOSE 9005
CMD ["/opt/xlapps/AEMGRBE/bin/agent-earth-mgr-backend", "-f", "/opt/xlconfigs/AEMGRBE/config.yaml"]
