FROM golang:1.12 as builder
WORKDIR /go/src/github.com/graphite-ng/carbon-relay-ng
RUN apt update && apt install go-bindata -y
COPY . .
RUN make

FROM alpine
RUN apk --update add --no-cache ca-certificates
COPY --from=builder /go/src/github.com/graphite-ng/carbon-relay-ng/carbon-relay-ng /bin/
VOLUME /conf
ADD examples/carbon-relay-ng.ini /conf/carbon-relay-ng.ini
RUN mkdir /var/spool/carbon-relay-ng
ENTRYPOINT ["/bin/carbon-relay-ng"]
CMD ["/conf/carbon-relay-ng.ini"]
