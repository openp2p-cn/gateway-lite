FROM debian:buster
RUN sed -i 's#http://deb.debian.org#http://mirrors.cloud.tencent.com#g' /etc/apt/sources.list \
    && sed -i 's#http://security.debian.org#http://mirrors.cloud.tencent.com#g' /etc/apt/sources.list \
    && apt-get update \
    && apt-get install -y ca-certificates curl
COPY openp2p-gateway /
COPY api.key /
COPY api.crt /
CMD ["/openp2p-gateway"]
