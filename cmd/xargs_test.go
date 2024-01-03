package cmd

import (
	"bufio"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestXargsFlagParse(t *testing.T) {
	cases := []struct {
		name             string
		input            []string
		expectedFlags    xargsFlags
		expectedRestArgs []string
	}{
		{"all flags set and command exists", []string{"-n", "3", "-P", "4", "-0", "--exit-on-error", "-I", "{}", "grep", "-l"}, xargsFlags{delimiter: zeroDelimiter, maxProcs: 4, maxArgs: 1, replacement: "{}", exitOnError: true}, []string{"grep", "-l"}},
		{"all long flags set and command exists", []string{"--max-args", "3", "--max-procs", "4", "-0", "grep", "-l"}, xargsFlags{delimiter: zeroDelimiter, maxProcs: 4, maxArgs: 3}, []string{"grep", "-l"}},
		{"no max procs and, command exists", []string{"-n", "3", "-0", "grep", "-l"}, xargsFlags{delimiter: zeroDelimiter, maxProcs: 1, maxArgs: 3}, []string{"grep", "-l"}},
		{"no max procs, no zero delimited and command exists", []string{"-n", "3", "grep", "-l"}, xargsFlags{delimiter: newLineDelimiter, maxProcs: 1, maxArgs: 3}, []string{"grep", "-l"}},
		{"only command exists", []string{"grep", "-l"}, xargsFlags{delimiter: newLineDelimiter, maxProcs: 1, maxArgs: 1}, []string{"grep", "-l"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newXargsFlags()
			got, err := f.parseFlags(c.input)
			if err != nil {
				t.Errorf("Error not expected here %s", err)
			}

			if *f != c.expectedFlags {
				t.Errorf("got %v want %v", *f, c.expectedFlags)
			}

			if !reflect.DeepEqual(got, c.expectedRestArgs) {
				t.Errorf("got %v want %v", got, c.expectedRestArgs)
			}
		})
	}
}

func TestXargsFlagsParseInvalid(t *testing.T) {
	cases := []struct {
		name  string
		input []string
		want  error
	}{
		{"invalid max-procs (not number) and command exists", []string{"-n", "3", "-P", "aaa", "-0", "grep", "-l"}, errInvalidArgument},
		{"invalid max-procs (missing) and command exists", []string{"-n", "3", "-P", "-0", "grep", "-l"}, errMissingArgument},
		{"command does not exists", []string{"-n", "3", "-0"}, errNoCommandSpecified},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newXargsFlags()
			_, err := f.parseFlags(c.input)

			if err == nil {
				t.Errorf("Error expected here")
			}

			if !errors.Is(err, c.want) {
				t.Errorf("got %v want %v", err, c.want)
			}

		})
	}
}

func TestPassBySingle(t *testing.T) {
	cases := []struct {
		name             string
		inputArgs        []string
		inputStdin       string
		inputReplacement string
		want             [][]string
	}{
		{
			"no replacement exists",
			[]string{"grep", "foo"},
			"file1.txt\nfile2.txt",
			"",
			[][]string{{"grep", "foo", "file1.txt"}, {"grep", "foo", "file2.txt"}},
		},
		{
			"one replacement exists",
			[]string{"mv", "{}", "/tmp/"},
			"file1.txt\nfile2.txt",
			"{}",
			[][]string{{"mv", "file1.txt", "/tmp/"}, {"mv", "file2.txt", "/tmp/"}},
		},
		{
			"multiple replacement exists",
			[]string{"mv", "{}", "{}.bck"},
			"file1.txt\nfile2.txt",
			"{}",
			[][]string{{"mv", "file1.txt", "file1.txt.bck"}, {"mv", "file2.txt", "file2.txt.bck"}},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			scanner := bufio.NewScanner(strings.NewReader(c.inputStdin))
			ch := make(chan []string, len(c.want))
			passBySingle(scanner, c.inputArgs, ch, nil, c.inputReplacement)

			got := make([][]string, 0, len(c.want))
			for v := range ch {
				got = append(got, v)
			}

			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
}

func TestPassByMultiple(t *testing.T) {
	cases := []struct {
		name         string
		inputArgs    []string
		inputStdin   string
		inputMaxArgs int
		want         [][]string
	}{
		{
			"less than maxArgs exists",
			[]string{"grep", "foo"},
			"file1.txt",
			2,
			[][]string{{"grep", "foo", "file1.txt"}},
		},
		{
			"multiple of maxArgs",
			[]string{"grep", "foo"},
			"file1.txt\nfile2.txt\nfile3.txt\nfile4.txt",
			2,
			[][]string{{"grep", "foo", "file1.txt", "file2.txt"}, {"grep", "foo", "file3.txt", "file4.txt"}},
		},
		{
			"non-multiple of maxArgs",
			[]string{"grep", "foo"},
			"file1.txt\nfile2.txt\nfile3.txt",
			2,
			[][]string{{"grep", "foo", "file1.txt", "file2.txt"}, {"grep", "foo", "file3.txt"}},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			scanner := bufio.NewScanner(strings.NewReader(c.inputStdin))
			ch := make(chan []string, len(c.want))
			passByMultiple(scanner, c.inputArgs, ch, nil, c.inputMaxArgs)

			got := make([][]string, 0, len(c.want))
			for v := range ch {
				got = append(got, v)
			}

			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
}
