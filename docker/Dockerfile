FROM alpine:latest
MAINTAINER Toni Moreno <toni.moreno@gmail.com>

ADD ./syncflux-last.tar.gz /

VOLUME ["/opt/syncflux/conf", "/opt/syncflux/log"]

EXPOSE 8090

WORKDIR /opt/syncflux
COPY ./syncflux.toml ./conf/
COPY ./start.sh /

ENTRYPOINT ["/start.sh"]
