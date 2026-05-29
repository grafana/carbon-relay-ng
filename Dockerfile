FROM alpine@sha256:4b7ce07002c69e8f3d704a9c5d6fd3053be500b7f1c69fc0d80990c2ad8dd412 AS builder
RUN apk --update add --no-cache ca-certificates
RUN mkdir /var/spool/carbon-relay-ng

# But the final image is distroless
FROM gcr.io/distroless/static-debian12@sha256:9c346e4be81b5ca7ff31a0d89eaeade58b0f95cfd3baed1f36083ddb47ca3160
COPY --from=builder /etc/ssl /etc/ssl
COPY --from=builder /var/spool /var/spool

VOLUME /conf
ADD examples/carbon-relay-ng.ini /conf/carbon-relay-ng.ini
ADD --chmod=555 carbon-relay-ng-linux-amd64 /bin/carbon-relay-ng

ENTRYPOINT ["/bin/carbon-relay-ng"]
CMD ["/conf/carbon-relay-ng.ini"]
