package cmd

import (
	"io/fs"
	"testing"
	"time"
)

func TestParseMmin(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  timeFilter
	}{
		{
			"exact time",
			"5",
			*newTimeFilter(parseTime(t, "2023.12.28 08:55"), exactly),
		},
		{
			"within last 5 minutes",
			"-5",
			*newTimeFilter(parseTime(t, "2023.12.28 08:55"), lessThan),
		},
		{
			"older than 5 minutes",
			"+5",
			*newTimeFilter(parseTime(t, "2023.12.28 08:55"), moreThan),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			now := parseTime(t, "2023.12.28 09:00")
			got, err := parseMmin(c.input, now)

			if err != nil {
				t.Errorf("Error not expected %s", err)
			}

			if got.filterType != c.want.filterType {
				t.Errorf("got %v want %v", *got, c.want)
			}

			if got.threshold != c.want.threshold {
				t.Errorf("got %v want %v", *got, c.want)
			}

		})
	}
}

func TestTimeFilterAccept(t *testing.T) {
	files := map[string]fileStat{
		"exactly_5_minutes_ago.txt":   {"exactly_5_minutes_ago.txt", parseTime(t, "2023.12.28 08:55")},
		"less_5_minutes_ago.txt":      {"less_5_minutes_ago.txt", parseTime(t, "2023.12.28 08:57")},
		"more_than_5_minutes_ago.txt": {"more_than_5_minutes_ago.txt", parseTime(t, "2023.12.28 08:50")},
	}

	statFunc := func(name string) (fs.FileInfo, error) {
		return files[name], nil
	}

	cases := []struct {
		name        string
		inputFile   fileStat
		inputFilter timeFilter
		want        bool
	}{
		{"exact time match", files["exactly_5_minutes_ago.txt"], timeFilter{parseTime(t, "2023.12.28 08:55"), exactly, statFunc}, true},
		{"exact time no match", files["less_5_minutes_ago.txt"], timeFilter{parseTime(t, "2023.12.28 08:55"), exactly, statFunc}, false},
		{"within last 5 minutes match", files["less_5_minutes_ago.txt"], timeFilter{parseTime(t, "2023.12.28 08:55"), lessThan, statFunc}, true},
		{"within last 5 minutes no match", files["more_than_5_minutes_ago.txt"], timeFilter{parseTime(t, "2023.12.28 08:55"), lessThan, statFunc}, false},
		{"older than 5 minutes match", files["more_than_5_minutes_ago.txt"], timeFilter{parseTime(t, "2023.12.28 08:55"), moreThan, statFunc}, true},
		{"older than 5 minutes no match", files["less_5_minutes_ago.txt"], timeFilter{parseTime(t, "2023.12.28 08:55"), moreThan, statFunc}, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.inputFilter.Accept(c.inputFile.name)
			if got != c.want {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
}

type fileStat struct {
	name    string
	modTime time.Time
}

func (fs fileStat) Name() string       { return fs.name }
func (fs fileStat) Size() int64        { return 0 }
func (fs fileStat) Mode() fs.FileMode  { return 0755 }
func (fs fileStat) ModTime() time.Time { return fs.modTime }
func (fs fileStat) Sys() any           { return nil }
func (fs fileStat) IsDir() bool        { return false }

func parseTime(t *testing.T, timestamp string) time.Time {
	t.Helper()
	pt, _ := time.Parse("2006.01.02 15:04", timestamp)
	return pt.Truncate(time.Minute)
}
