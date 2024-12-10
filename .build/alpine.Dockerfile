FROM docker.io/library/golang:1.22-alpine

WORKDIR /build

COPY . .

ENTRYPOINT ["go", "build", "-o", "/out/ssbak"]