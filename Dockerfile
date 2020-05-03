FROM golang:1.13.4-alpine as builder

RUN apk add --no-cache gcc g++ bash git

WORKDIR $GOPATH/src/github.com/toni-moreno/syncflux

COPY go.mod go.sum ./

COPY pkg pkg
COPY .git .git
COPY build.go ./

RUN go run build.go  build

FROM alpine:latest
MAINTAINER Toni Moreno <toni.moreno@gmail.com>



VOLUME ["/opt/syncflux/conf", "/opt/syncflux/log"]

EXPOSE 8090

COPY --from=builder /go/src/github.com/toni-moreno/syncflux/bin/syncflux ./bin/

WORKDIR /opt/syncflux
COPY ./conf/sample.syncflux.toml ./conf/syncflux.toml

ENTRYPOINT ["/bin/syncflux"]
