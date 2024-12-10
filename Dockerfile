FROM docker.io/alpine:3
LABEL maintainer="i-segura <github@m.isu101.com>"
LABEL org.opencontainers.image.authors="i-segura <github@m.isu101.com>"
LABEL org.opencontainers.image.source="https://github.com/stupid-simple/backup"
LABEL org.opencontainers.image.description="Simple automated archiver"

RUN apk add --no-cache tini

COPY ./ssbak /usr/local/bin/ssbak

ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/ssbak"]
