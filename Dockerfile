FROM golang:1.13 AS builder
RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN make build

FROM scratch
WORKDIR /
COPY --from=builder /build/check_mount_exporter /check_mount_exporter
ENTRYPOINT ["/check_mount_exporter"]
