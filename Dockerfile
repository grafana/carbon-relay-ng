FROM alpine:3.17.0
RUN apk upgrade --no-cache
RUN apk --update add --no-cache ca-certificates
ADD carbon-relay-ng /bin/
VOLUME /conf
ADD examples/carbon-relay-ng.ini /conf/carbon-relay-ng.ini
RUN mkdir /var/spool/carbon-relay-ng
ENTRYPOINT ["/bin/carbon-relay-ng"]
CMD ["/conf/carbon-relay-ng.ini"]
