package cmd

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParsePasswdFile(t *testing.T) {
	input := strings.NewReader(`root:x:0:0:root:/root:/bin/bash
bin:x:1:1:bin:/bin:/sbin/nologin
daemon:x:2:2:daemon:/sbin:/sbin/nologin
adm:x:3:4:adm:/var/adm:/sbin/nologin
`)
	got := parsePasswdFile(input)
	want := map[string]string{
		"0": "root", "1": "bin", "2": "daemon", "3": "adm",
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestParseProcStatusFile(t *testing.T) {
	f, err := os.Open("testdata/procfs_status")
	require.NoError(t, err)
	defer f.Close()

	got, err := parseProcStatus(f)
	require.NoError(t, err)

	want := procStatus{
		name:      "gsd-media-keys",
		pid:       "2553",
		parentPid: "1674",
		uid:       "42",
		gid:       "45",
	}

	if !reflect.DeepEqual(*got, want) {
		t.Errorf("got %v want %v", *got, want)
	}
}

func TestFormatCpuTim(t *testing.T) {
	cases := []struct {
		name  string
		input int
		want  string
	}{
		{"less than 1 minute", 5, "00:00:05"},
		{"less than 1 hour", 75, "00:01:15"},
		{"more than 1 hour", 3665, "01:01:05"},
		{"25 hours 3 minutes 5 seconds", 90185, "25:03:05"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := formatCpuTime(c.input)

			if got != c.want {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
}
