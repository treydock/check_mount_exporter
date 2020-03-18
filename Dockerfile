FROM scratch
ARG ARCH="amd64"
ARG OS="linux"
COPY .build/${OS}-${ARCH}/check_mount_exporter /check_mount_exporter
EXPOSE 9304
ENTRYPOINT ["/check_mount_exporter"]
