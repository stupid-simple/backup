FROM docker.io/busybox:stable-glibc

LABEL maintainer="i-segura <github@m.isu101.com>"
LABEL org.opencontainers.image.authors="i-segura <github@m.isu101.com>"
LABEL org.opencontainers.image.source="https://github.com/stupid-simple/backup"
LABEL org.opencontainers.image.description="Simple automated archiver"

ENV TINI_VERSION=v0.19.0
ADD https://github.com/krallin/tini/releases/download/${TINI_VERSION}/tini /bin/tini
RUN chmod +x /bin/tini

COPY ./ssbak /usr/local/bin/ssbak

ENTRYPOINT ["/bin/tini", "--", "/usr/local/bin/ssbak"]
