package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	linuxproc "github.com/c9s/goprocinfo/linux"
	fstab "github.com/deniswernert/go-fstab"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/yaml.v2"
)

var (
	configMountpoints = kingpin.Flag("config.mountpoints", "Comma separated list of mountpoints to check").Default("").String()
	configExclude     = kingpin.Flag("config.exclude", "Comma separated list of mountpoints to exclude").Default("").String()
	listenAddress     = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":9304").String()
	procMounts        = "/proc/mounts"
	fstabPath         = "/etc/fstab"
)

type Config struct {
	mountpoints []string
	exclude     []string
}

type CheckMountMetric struct {
	mountpoint string
	status     float64
}

type Exporter struct {
	config  *Config
	status  *prometheus.Desc
	success *prometheus.Desc
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

func (c *Config) LoadConfig(configFile string) error {
	yamlFile, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Errorf("Error reading config file: %s", err)
		return err
	}
	if err := yaml.Unmarshal(yamlFile, c); err != nil {
		log.Errorf("Error parsing config file: %s", err)
		return err
	}
	log.Infof("Loaded config file %s mounts %d", configFile, len(c.mountpoints))
	return nil
}

func (c *Config) ParseFSTab() error {
	if exists := fileExists(fstabPath); !exists {
		return fmt.Errorf("%s does not exist", fstabPath)
	}
	mounts, err := fstab.ParseFile(fstabPath)
	if err != nil {
		return err
	}
	for _, m := range mounts {
		if sliceContains(c.exclude, m.File) {
			continue
		}
		c.mountpoints = append(c.mountpoints, m.File)
	}
	return nil
}

func NewExporter(c *Config) *Exporter {
	return &Exporter{
		config:  c,
		status:  prometheus.NewDesc("check_mount_status", "Mount point status, 1=mounted 0=not mounted", []string{"mountpoint"}, nil),
		success: prometheus.NewDesc("check_mount_success", "Exporter status, 1=successful 0=errors", nil, nil),
	}
}

func (e *Exporter) collect() ([]CheckMountMetric, error) {
	var mountpoints []string
	var metrics []CheckMountMetric
	if e.config.mountpoints == nil {
		if err := e.config.ParseFSTab(); err != nil {
			return nil, fmt.Errorf("Unable to load config from %s: %s", fstabPath, err.Error())
		}
	}
	log.Debugf("Collecting mountpoints: %v", e.config.mountpoints)
	mounts, err := linuxproc.ReadMounts(procMounts)
	if err != nil {
		return nil, err
	}
	for _, mount := range mounts.Mounts {
		mountpoints = append(mountpoints, mount.MountPoint)
	}
	for _, m := range e.config.mountpoints {
		mounted := sliceContains(mountpoints, m)
		var status float64
		if mounted {
			status = 1
		} else {
			status = 0
		}
		metric := CheckMountMetric{mountpoint: m, status: status}
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
		ch <- prometheus.MustNewConstMetric(e.status, prometheus.GaugeValue, m.status, m.mountpoint)
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

	config := &Config{}
	if *configMountpoints != "" {
		mountpoints := strings.Split(*configMountpoints, ",")
		config.mountpoints = mountpoints
	}
	if *configExclude != "" {
		exclude := strings.Split(*configExclude, ",")
		config.exclude = exclude
	}

	exporter := NewExporter(config)
	prometheus.MustRegister(exporter)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>check_mount Exporter</title></head>
             <body>
             <h1>check_mount Exporter</h1>
             <p><a href='` + metricsEndpoint + `'>Metrics</a></p>
             </body>
             </html>`))
	})
	http.Handle(metricsEndpoint, promhttp.Handler())
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
