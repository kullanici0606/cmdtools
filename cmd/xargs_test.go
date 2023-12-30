package cmd

import (
	"errors"
	"reflect"
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
