FROM alpine:3.18.3
RUN apk upgrade --no-cache
RUN apk --update add --no-cache ca-certificates
ADD carbon-relay-ng-linux-amd64 /bin/carbon-relay-ng
VOLUME /conf
ADD examples/carbon-relay-ng.ini /conf/carbon-relay-ng.ini
RUN mkdir /var/spool/carbon-relay-ng
ENTRYPOINT ["/bin/carbon-relay-ng"]
CMD ["/conf/carbon-relay-ng.ini"]
