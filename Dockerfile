FROM docker.io/alpine:3
LABEL maintainer="i-segura <github@m.isu101.com>"

ENV TZ=Etc/UTC
RUN apk add --no-cache tzdata shadow su-exec \
        && addgroup -g 1000 app \
        && adduser -D -H -G app -u 1000 app

COPY ./entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

COPY ./entrypoint.sh /root/entrypoint.sh
COPY ./ssbak /usr/local/bin/ssbak

ENTRYPOINT ["/root/entrypoint.sh", "/usr/local/sbin/ssbak"]
