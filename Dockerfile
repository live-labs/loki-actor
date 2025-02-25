FROM alpine:latest

RUN apk add --no-cache \
    ca-certificates \
    curl

COPY bin/loki-actor /bin/loki-actor

ENTRYPOINT ["/bin/loki-actor", "-config", "/etc/loki-actor/config.yml"]