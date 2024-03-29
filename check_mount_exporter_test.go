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
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	kingpin "github.com/alecthomas/kingpin/v2"
	"github.com/go-kit/log"
)

const (
	address = "localhost:19304"
)

func TestMain(m *testing.M) {
	w := log.NewSyncWriter(os.Stderr)
	logger := log.NewLogfmtLogger(w)
	go func() {
		http.Handle("/metrics", metricsHandler(logger))
		err := http.ListenAndServe(address, nil)
		if err != nil {
			os.Exit(1)
		}
	}()
	time.Sleep(1 * time.Second)

	exitVal := m.Run()

	os.Exit(exitVal)
}

func TestCollect(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	rootfsPathTmp, err := os.MkdirTemp(os.TempDir(), "check_mounts")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(rootfsPathTmp)
	proc := rootfsPathTmp + "/proc"
	rootfsPath = &rootfsPathTmp
	mockedProcMounts := `/dev/root / ext4 rw,noatime 0 0
/dev/mapper/vg-lv_home /home ext4 ro,noatime 0 0
/dev/mapper/vg-lv_var /var ext4 rw,noatime 0 0
/dev/mapper/vg-lv_tmp /tmp ext4 rw,noatime 0 0
`
	if err := os.MkdirAll(proc, 0755); err != nil {
		t.Fatalf("MkdirAll %s: %s", proc, err)
	}
	mounts := proc + "/mounts"
	if err := os.WriteFile(mounts, []byte(mockedProcMounts), 0644); err != nil {
		t.Fatal(err)
	}
	w := log.NewSyncWriter(os.Stderr)
	logger := log.NewLogfmtLogger(w)
	exporter := NewExporter([]string{"/var", "/home", "/dne"}, logger)
	metrics, err := exporter.collect()
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
		return
	}
	if len(metrics) != 3 {
		t.Errorf("Unexpected length of metrics, expected 3 got %d", len(metrics))
	}
	if val := metrics[0].status; val != 1 {
		t.Errorf("Unexpected status, got %v", val)
	}
	if val := metrics[0].rw; val != "rw" {
		t.Errorf("Unexpected status, got %v", val)
	}
	if val := metrics[1].status; val != 1 {
		t.Errorf("Unexpected status, got %v", val)
	}
	if val := metrics[1].rw; val != "ro" {
		t.Errorf("Unexpected status, got %v", val)
	}
	if val := metrics[2].status; val != 0 {
		t.Errorf("Unexpected status, got %v", val)
	}
	if val := metrics[2].rw; val != "" {
		t.Errorf("Unexpected status, got %v", val)
	}
}

func TestParseFSTab(t *testing.T) {
	if _, err := kingpin.CommandLine.Parse([]string{}); err != nil {
		t.Fatal(err)
	}
	rootfsPathTmp, err := os.MkdirTemp(os.TempDir(), "check_mounts")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(rootfsPathTmp)
	etc := rootfsPathTmp + "/etc"
	rootfsPath = &rootfsPathTmp
	mocked_fstab := `proc            /proc           proc    defaults          0       0
LABEL=swap      swap    swap    defaults        0       0
PARTUUID=6c586e13-01  /boot           ext3    defaults          0       2
PARTUUID=6c586e13-02  /               ext4    defaults,noatime  0       1
/dev/vg/lv_var       /var            ext4    defaults,noatime 0 0
/dev/vg/lv_puppet    /etc/puppet     ext4    defaults,noatime 0 0
/dev/vg/lv_home      /home           ext4    defaults,noatime 0 0
/dev/vg/lv_tmp       /tmp            ext4    defaults,noatime 0 0
`
	if err := os.MkdirAll(etc, 0755); err != nil {
		t.Fatalf("MkdirAll %s: %s", etc, err)
	}
	fstab := etc + "/fstab"
	if err := os.WriteFile(fstab, []byte(mocked_fstab), 0644); err != nil {
		t.Fatal(err)
	}
	w := log.NewSyncWriter(os.Stderr)
	logger := log.NewLogfmtLogger(w)
	exporter := NewExporter(nil, logger)
	mountpoints, err := exporter.ParseFSTab()
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
		return
	}
	if val := len(mountpoints); val != 6 {
		t.Errorf("Unexpected number of mountpoints: %d", val)
	}
}

func TestMetricsHandler(t *testing.T) {
	rootfsPathTmp, err := os.MkdirTemp(os.TempDir(), "check_mounts")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(rootfsPathTmp)
	etc := rootfsPathTmp + "/etc"
	proc := rootfsPathTmp + "/proc"
	rootfsPath = &rootfsPathTmp
	mocked_fstab := `proc            /proc           proc    defaults          0       0
LABEL=swap      swap    swap    defaults        0       0
PARTUUID=6c586e13-01  /boot           ext3    defaults          0       2
PARTUUID=6c586e13-02  /               ext4    defaults,noatime  0       1
/dev/vg/lv_var       /var            ext4    defaults,noatime 0 0
/dev/vg/lv_puppet    /etc/puppet     ext4    defaults,noatime 0 0
/dev/vg/lv_home      /home           ext4    defaults,noatime 0 0
/dev/vg/lv_tmp       /tmp            ext4    defaults,noatime 0 0
`
	mockedProcMounts := `/dev/root / ext4 rw,noatime 0 0
/dev/mapper/vg-lv_home /home ext4 ro,noatime 0 0
/dev/mapper/vg-lv_var /var ext4 rw,noatime 0 0
/dev/mapper/vg-lv_tmp /tmp ext4 rw,noatime 0 0
`
	if err := os.MkdirAll(proc, 0755); err != nil {
		t.Fatalf("MkdirAll %s: %s", proc, err)
	}
	if err := os.MkdirAll(etc, 0755); err != nil {
		t.Fatalf("MkdirAll %s: %s", proc, err)
	}
	fstab := etc + "/fstab"
	mounts := proc + "/mounts"
	if err := os.WriteFile(mounts, []byte(mockedProcMounts), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fstab, []byte(mocked_fstab), 0644); err != nil {
		t.Fatal(err)
	}
	body, err := queryExporter()
	if err != nil {
		t.Fatalf("Unexpected error GET /metrics: %s", err.Error())
	}
	if !strings.Contains(body, "check_mount_status{mountpoint=\"/var\",rw=\"rw\"} 1") {
		t.Errorf("Unexpected value for check_mount_status")
	}
}

func queryExporter() (string, error) {
	resp, err := http.Get(fmt.Sprintf("http://%s/metrics", address))
	if err != nil {
		return "", err
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if err := resp.Body.Close(); err != nil {
		return "", err
	}
	if want, have := http.StatusOK, resp.StatusCode; want != have {
		return "", fmt.Errorf("want /metrics status code %d, have %d. Body:\n%s", want, have, b)
	}
	return string(b), nil
}
