// Copyright 2020 Trey Dockendorf
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	linuxproc "github.com/c9s/goprocinfo/linux"
	fstab "github.com/deniswernert/go-fstab"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	defExcludeMountpoints = "^/(dev|proc|sys|var/lib/docker/.+)($|/)"
	defExcludeFSTypes     = "^(proc|procfs|sysfs|swap)$"
)

var (
	defRootfs                = "/"
	configMountpoints        = kingpin.Flag("config.mountpoints", "Comma separated list of mountpoints to check").Default("").String()
	configExcludeMountpoints = kingpin.Flag("config.exclude.mountpoints", "Regex of mountpoints to exclude").Default(defExcludeMountpoints).String()
	configExcludeFSTypes     = kingpin.Flag("config.exclude.fs-types", "Regex of filesystem types to exclude").Default(defExcludeFSTypes).String()
	rootfsPath               = kingpin.Flag("path.rootfs", "Path to root filesystem").Default(defRootfs).String()
	listenAddress            = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":9304").String()
	disableExporterMetrics   = kingpin.Flag("web.disable-exporter-metrics", "Exclude metrics about the exporter (promhttp_*, process_*, go_*)").Default("false").Bool()
)

type CheckMountMetric struct {
	mountpoint string
	status     float64
	rw         string
}

type Exporter struct {
	mountpoints               []string
	excludeMountpointsPattern *regexp.Regexp
	excludeFSTypesPattern     *regexp.Regexp
	status                    *prometheus.Desc
	success                   *prometheus.Desc
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func sliceContains(slice []string, str string) bool {
	for _, s := range slice {
		if str == s {
			return true
		}
	}
	return false
}

func rootfsStripPrefix(path string) string {
	if *rootfsPath == "/" {
		return path
	}
	stripped := strings.TrimPrefix(path, *rootfsPath)
	if stripped == "" {
		return "/"
	}
	return stripped
}

func NewExporter(mountpoints []string) *Exporter {
	excludeMountpointsPattern := regexp.MustCompile(*configExcludeMountpoints)
	excludeFSTypesPattern := regexp.MustCompile(*configExcludeFSTypes)
	return &Exporter{
		mountpoints:               mountpoints,
		excludeMountpointsPattern: excludeMountpointsPattern,
		excludeFSTypesPattern:     excludeFSTypesPattern,
		status:                    prometheus.NewDesc("check_mount_status", "Mount point status, 1=mounted 0=not mounted", []string{"mountpoint", "rw"}, nil),
		success:                   prometheus.NewDesc("check_mount_success", "Exporter status, 1=successful 0=errors", nil, nil),
	}
}

func (e *Exporter) ParseFSTab() ([]string, error) {
	var mountpoints []string
	fstabPath := filepath.Join(*rootfsPath, "etc/fstab")
	if exists := fileExists(fstabPath); !exists {
		return nil, fmt.Errorf("%s does not exist", fstabPath)
	}
	mounts, err := fstab.ParseFile(fstabPath)
	if err != nil {
		return nil, err
	}
	for _, m := range mounts {
		if e.excludeMountpointsPattern.MatchString(m.File) || e.excludeFSTypesPattern.MatchString(m.VfsType) {
			log.Debugf("Ignoring mount point %s", m.File)
			continue
		}
		mountpoints = append(mountpoints, m.File)
	}
	return mountpoints, nil
}

func (e *Exporter) collect() ([]CheckMountMetric, error) {
	var mountpoints []string
	var mountpointsRW []string
	var mountpointsRO []string
	var metrics []CheckMountMetric
	if e.mountpoints == nil {
		if mountpoints, err := e.ParseFSTab(); err != nil {
			return nil, fmt.Errorf("Unable to load from fstab: %s", err.Error())
		} else {
			e.mountpoints = mountpoints
		}
	}
	log.Debugf("Collecting mountpoints: %v", e.mountpoints)
	procMounts := filepath.Join(*rootfsPath, "proc/mounts")
	log.Debugf("Parsing /proc/mounts from %s", procMounts)
	mounts, err := linuxproc.ReadMounts(procMounts)
	if err != nil {
		return nil, err
	}
	for _, mount := range mounts.Mounts {
		mountpoint := rootfsStripPrefix(mount.MountPoint)
		log.Debugf("Found mount %s", mountpoint)
		mountpoints = append(mountpoints, mountpoint)
		if strings.Contains(mount.Options, "rw,") {
			mountpointsRW = append(mountpointsRW, mountpoint)
		} else if strings.Contains(mount.Options, "ro,") {
			mountpointsRO = append(mountpointsRO, mountpoint)
		}
	}
	for _, m := range e.mountpoints {
		var rw_value string
		var status float64
		mounted := sliceContains(mountpoints, m)
		rw := sliceContains(mountpointsRW, m)
		ro := sliceContains(mountpointsRO, m)
		if mounted {
			status = 1
		} else {
			status = 0
		}
		if rw {
			rw_value = "rw"
		} else if ro {
			rw_value = "ro"
		}
		metric := CheckMountMetric{mountpoint: m, status: status, rw: rw_value}
		metrics = append(metrics, metric)
	}
	return metrics, nil
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.status
	ch <- e.success
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	metrics, err := e.collect()
	if err != nil {
		ch <- prometheus.MustNewConstMetric(e.success, prometheus.GaugeValue, 0)
	} else {
		ch <- prometheus.MustNewConstMetric(e.success, prometheus.GaugeValue, 1)
	}
	for _, m := range metrics {
		ch <- prometheus.MustNewConstMetric(e.status, prometheus.GaugeValue, m.status, m.mountpoint, m.rw)
	}
}

func metricsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		registry := prometheus.NewRegistry()

		var mountpoints []string
		if *configMountpoints != "" {
			mountpoints = strings.Split(*configMountpoints, ",")
		}

		exporter := NewExporter(mountpoints)
		registry.MustRegister(exporter)

		gatherers := prometheus.Gatherers{registry}
		if !*disableExporterMetrics {
			gatherers = append(gatherers, prometheus.DefaultGatherer)
		}

		// Delegate http serving to Prometheus client library, which will call collector.Collect.
		h := promhttp.HandlerFor(gatherers, promhttp.HandlerOpts{})
		h.ServeHTTP(w, r)
	}
}

func main() {
	metricsEndpoint := "/metrics"
	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("check_mount_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	log.Infoln("Starting check_mount_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())
	log.Infoln("Starting Server:", *listenAddress)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		//nolint:errcheck
		w.Write([]byte(`<html>
             <head><title>check_mount Exporter</title></head>
             <body>
             <h1>check_mount Exporter</h1>
             <p><a href='` + metricsEndpoint + `'>Metrics</a></p>
             </body>
             </html>`))
	})
	http.Handle(metricsEndpoint, metricsHandler())
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
