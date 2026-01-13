ARG DOCKER_REP_PATH=""
FROM ${DOCKER_REP_PATH}ubuntu:22.04

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates \
  && update-ca-certificates \
  && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /opt/xlapps/AEMgrBE/bin \
  && mkdir -p /opt/xltmp/AEMgrBE/tmp \
  && mkdir -p /opt/xldata/AEMgrBE/data

WORKDIR /opt/xlapps/AEMgrBE/bin

COPY bin/agent-earth-manager /opt/xlapps/AEMgrBE/bin/agent-earth-manager

RUN chmod +x /opt/xlapps/AEMgrBE/bin/agent-earth-manager

EXPOSE 9005
CMD ["/opt/xlapps/AEMgrBE/bin/agent-earth-manager", "-f", "/opt/xlconfigs/AEMgrBE/admin-api.yaml"]
