FROM docker.io/library/golang:1.22-alpine AS builder

RUN apk add --no-cache build-base

COPY . /builder/

WORKDIR /builder

RUN GOOS=${BUILDOS} GOARCH=${BUILDARCH} go build -o ssbak

FROM docker.io/alpine:3
LABEL maintainer="i-segura <github@m.isu101.com>"

ENV TZ=Etc/UTC
RUN apk add --no-cache tzdata shadow su-exec \
        && addgroup -g 1000 app \
        && adduser -D -H -G app -u 1000 app

COPY ./entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

COPY --from=builder /builder/ssbak /usr/local/sbin/ssbak

ENTRYPOINT ["/usr/local/bin/entrypoint.sh", "/usr/local/sbin/ssbak"]
