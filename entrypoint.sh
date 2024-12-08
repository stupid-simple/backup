#!/bin/sh

set -eu

UID=${UID:-1000}
GID=${GID:-1000}
TZ=${TZ:-Etc/UTC}

ln -sf "/usr/share/zoneinfo/$TZ" /etc/localtime && echo "$TZ" > /etc/timezone

if ! getent group ${GID} > /dev/null 2>&1; then
    addgroup -g "${GID}" ssbak
fi

if ! getent passwd ${UID} > /dev/null 2>&1; then
    adduser -D -H -u "${UID}" ssbak
    usermod -g "${GID}" ssbak
fi

exec su-exec ${UID}:${GID} "$@"