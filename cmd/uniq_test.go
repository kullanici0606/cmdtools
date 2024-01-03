package cmd

import (
	"reflect"
	"strings"
	"testing"
)

func TestRunUniq(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  map[string]int
	}{
		{
			"all unique",
			"1\n2\n3",
			map[string]int{"1": 1, "2": 1, "3": 1, "": 0},
		},
		{
			"some duplicates",
			"1\n2\n2\n2\n3\n3",
			map[string]int{"1": 1, "2": 3, "3": 2, "": 0},
		},
		{
			"single input",
			"1",
			map[string]int{"1": 1, "": 0},
		},
		{
			"with empty lines",
			"1\n\n\n\n3\n3",
			map[string]int{"1": 1, "": 3, "3": 2},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := make(map[string]int)
			fn := func(txt string, count int) {
				got[txt] = count
			}

			runUniq(strings.NewReader(c.input), fn)

			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %v want %v", got, c.want)
			}

		})
	}
}
