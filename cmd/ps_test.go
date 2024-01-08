package cmd

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

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
		input int64
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

func TestTruncateUsername(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"short user name", "root", "root"},
		{"exactly 8 char username", "username", "username"},
		{"long user name", "storagemgmnt", "storage+"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := truncateUsername(c.input)
			if got != c.want {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
}

func TestParseCommandLine(t *testing.T) {
	f, err := os.Open("testdata/cmdline")
	require.NoError(t, err)

	got, err := parseCommandLine(f)
	require.NoError(t, err)

	want := "/home/user/go/bin/gopls -mode=stdio"
	if got != want {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestParseBootTime(t *testing.T) {
	now, err := time.Parse("02-01-2006 15:04:05 -07", "08-01-2024 09:02:47 +03")
	require.NoError(t, err)

	// /proc/uptime ends with a new line
	r := strings.NewReader("2142995.32 15273180.25\n")

	got, err := parseBootTime(r, now)
	require.NoError(t, err)

	want, err := time.Parse("02-01-2006 15:04:05 -07", "14-12-2023 13:46:12 +03")
	require.NoError(t, err)

	if got != want {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestParseProcStat(t *testing.T) {
	const clocktick = 100
	cases := []struct {
		name          string
		inputFile     string
		wantCpuTime   int64
		wantStartTime int64
	}{
		{"regular", "testdata/stat", (165404 + 17778) / clocktick, 171253092 / clocktick},
		{"process name with space", "testdata/stat_with_space_in_process_name", (123123 + 56464) / 100, 214505001 / 100},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f, err := os.Open(c.inputFile)
			require.NoError(t, err)
			defer f.Close()

			gotCpuTime, gotStartTime, err := parseProcStat(f, clocktick)
			require.NoError(t, err)
			if gotCpuTime != c.wantCpuTime {
				t.Errorf("got %v want %v", gotCpuTime, c.wantCpuTime)
			}

			if gotStartTime != c.wantStartTime {
				t.Errorf("got %v want %v", gotStartTime, c.wantStartTime)
			}
		})
	}
}
