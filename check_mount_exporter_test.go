package main

import (
	"github.com/Flaque/filet"
	"testing"
)

func TestCollect(t *testing.T) {
	procMounts = "/tmp/proc-mounts"
	mockedProcMounts := `/dev/root / ext4 rw,noatime 0 0
/dev/mapper/vg-lv_home /home ext4 rw,noatime 0 0
/dev/mapper/vg-lv_var /var ext4 rw,noatime 0 0
/dev/mapper/vg-lv_tmp /tmp ext4 rw,noatime 0 0
`
	filet.File(t, "/tmp/proc-mounts", mockedProcMounts)
	defer filet.CleanUp(t)
	config := &Config{mountpoints: []string{"/var", "/dne"}}
	exporter := NewExporter(config)
	metrics, err := exporter.collect()
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if len(metrics) != 2 {
		t.Errorf("Unexpected length of metrics, expected 2 got %d", len(metrics))
	}
	if val := metrics[0].status; val != 1 {
		t.Errorf("Unexpected status, got %v", val)
	}
	if val := metrics[1].status; val != 0 {
		t.Errorf("Unexpected status, got %v", val)
	}
}

func TestParseFSTab(t *testing.T) {
	fstabPath = "/tmp/fstab"
	mocked_fstab := `proc            /proc           proc    defaults          0       0
PARTUUID=6c586e13-01  /boot           vfat    defaults          0       2
PARTUUID=6c586e13-02  /               ext4    defaults,noatime  0       1
/dev/vg/lv_var       /var            ext4    defaults,noatime 0 0
/dev/vg/lv_puppet    /etc/puppet     ext4    defaults,noatime 0 0
/dev/vg/lv_home      /home           ext4    defaults,noatime 0 0
/dev/vg/lv_tmp       /tmp            ext4    defaults,noatime 0 0
`
	filet.File(t, "/tmp/fstab", mocked_fstab)
	defer filet.CleanUp(t)
	config := &Config{}
	err := config.ParseFSTab()
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if val := len(config.mountpoints); val != 7 {
		t.Errorf("Unexpected number of mountpoints: %d", val)
	}
}
