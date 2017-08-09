FROM alpine:latest

MAINTAINER Edward Muller <edward@heroku.com>

WORKDIR "/opt"

ADD .docker_build/link-to-download /opt/bin/link-to-download
ADD ./templates /opt/templates
ADD ./static /opt/static

CMD ["/opt/bin/link-to-download"]

