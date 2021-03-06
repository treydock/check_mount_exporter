# check_mount Prometheus exporter

[![Build Status](https://circleci.com/gh/treydock/check_mount_exporter/tree/master.svg?style=shield)](https://circleci.com/gh/treydock/check_mount_exporter)
[![GitHub release](https://img.shields.io/github/v/release/treydock/check_mount_exporter?include_prereleases&sort=semver)](https://github.com/treydock/check_mount_exporter/releases/latest)
![GitHub All Releases](https://img.shields.io/github/downloads/treydock/check_mount_exporter/total)
[![codecov](https://codecov.io/gh/treydock/check_mount_exporter/branch/master/graph/badge.svg)](https://codecov.io/gh/treydock/check_mount_exporter)

# Check mount Prometheus exporter

The `check_mount_exporter` produces metrics about mount points mount status and if that mountpoint is read-only or read-write.

This exporter by default listens on port `9304` and all metrics are exposed via the `/metrics` endpoint.

Example of metrics exposed by this exporter:

```
check_mount_status{mountpoint="/",rw="rw"} 1
check_mount_status{mountpoint="/boot",rw="rw"} 1
check_mount_status{mountpoint="/opt",rw="rw"} 1
check_mount_status{mountpoint="/tmp",rw="rw"} 1
check_mount_status{mountpoint="/var",rw="rw"} 1
```

# Usage

If the exporter is launched without `--config.mountpoints` then `/etc/fstab` will be parsed to identify which mountpoints to produce metrics for.

When parsing `/etc/fstab` you can exclude mountpoints using the `--config.exclude.mountpoints` and `--config.exclude.fs-types` flags.

The value for `--config.mountpoints` is comma separated while the exclude flags expect regular expressions.

## Docker

Example of running the Docker container

```
docker run -d -p 9304:9304 -v "/:/host:ro,rslave" treydock/check_mount_exporter --path.rootfs=/host
```

## Install

Download the [latest release](https://github.com/treydock/check_mount_exporter/releases)

## Build from source

To produce the `check_mount_exporter` binary:

```
make build
```

Or

```
go get github.com/treydock/check_mount_exporter
```
