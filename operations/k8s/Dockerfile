FROM alpine@sha256:4b7ce07002c69e8f3d704a9c5d6fd3053be500b7f1c69fc0d80990c2ad8dd412 AS builder
RUN apk --update add --no-cache ca-certificates
RUN mkdir /var/spool/carbon-relay-ng

RUN apk --update add --no-cache curl

RUN curl -sL \
    https://github.com/kelseyhightower/confd/releases/download/v0.16.0/confd-0.16.0-linux-amd64 \
        > /usr/local/bin/confd && \
    chmod a+x /usr/local/bin/confd && \
    chmod +x /usr/local/bin/confd

# krajorama: this check is to verify the checksum based on the one seen on 29th May 2026.
RUN echo "255d2559f3824dd64df059bdc533fd6b697c070db603c76aaf8d1d5e6b0cc334  /usr/local/bin/confd" | sha256sum -c -

# Pull /bin/sh from the busybox image rather than from alpine: alpine's
# /bin/sh is a symlink to /bin/busybox, which would dangle in the distroless
# image. The busybox-uclibc image ships a self-contained, statically linked
# binary that works standalone.
FROM busybox:1.38.0-uclibc@sha256:eea4ff5612c911abd1d0e9ed47ba642547b01c3490877d9c1bb5fd6346462da4 AS busybox

# But the final image is distroless
FROM gcr.io/distroless/static-debian12@sha256:20bc6c0bc4d625a22a8fde3e55f6515709b32055ef8fb9cfbddaa06d1760f838
COPY --from=builder /etc/ssl /etc/ssl
COPY --from=builder /var/spool /var/spool
COPY --from=builder /usr/local/bin/confd /usr/local/bin/confd
COPY --from=busybox /bin/sh /bin/sh
COPY --from=busybox /bin/busybox /bin/busybox

ENV INSTANCE=carbon

ADD --chmod=555 carbon-relay-ng-linux-amd64 /bin/carbon-relay-ng
COPY entrypoint.sh /usr/bin/entrypoint.sh
COPY templates/* /etc/confd/templates/
COPY conf.d/* /etc/confd/conf.d/
COPY storage-schemas.conf /etc/
COPY storage-aggregation.conf /etc/

ENTRYPOINT ["/usr/bin/entrypoint.sh"]
CMD ["/bin/carbon-relay-ng", "/etc/carbon-relay-ng.ini"]
