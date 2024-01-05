/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

// uniqCmd represents the uniq command
var uniqCmd = &cobra.Command{
	Use:   "uniq",
	Short: "Filter adjacent matching lines from INPUT (or standard input)",
	Run: func(cmd *cobra.Command, args []string) {
		executeUniq(args)
	},
}

type flags struct {
	count      bool
	duplicate  bool
	onlyUnique bool
}

var globalFlags flags

func init() {
	RootCmd.AddCommand(uniqCmd)
	uniqCmd.Flags().BoolVarP(&globalFlags.count, "count", "c", false, "prefix lines by the number of occurrences")
	uniqCmd.Flags().BoolVarP(&globalFlags.duplicate, "repeated", "d", false, "only print duplicate lines, one for each group")
	uniqCmd.Flags().BoolVarP(&globalFlags.onlyUnique, "unique", "u", false, "only print unique lines")
	uniqCmd.MarkFlagsMutuallyExclusive("count", "repeated", "unique")
}

func executeUniq(args []string) {
	fn := printAll
	switch {
	case globalFlags.count:
		fn = printCounts
	case globalFlags.duplicate:
		fn = printDuplicates
	case globalFlags.onlyUnique:
		fn = printUniques
	default:
		fn = printAll
	}

	if len(args) == 0 {
		runUniq(os.Stdin, fn)
		return
	}

	f, err := os.Open(args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open input %s %s", args[1], err)
		return
	}
	defer f.Close()
	runUniq(f, fn)
}

func runUniq(r io.Reader, fn func(string, int)) {
	prev := ""
	prevcount := 0
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if line == prev {
			prevcount++
			continue
		}

		fn(prev, prevcount)
		prev = line
		prevcount = 1
	}

	fn(prev, prevcount)
}

func printAll(txt string, count int) {
	if count > 0 {
		fmt.Println(txt)
	}
}

func printCounts(txt string, count int) {
	if count > 0 {
		fmt.Printf("%d\t%s\n", count, txt)
	}
}

func printDuplicates(txt string, count int) {
	if count > 1 {
		fmt.Println(txt)
	}
}

func printUniques(txt string, count int) {
	if count > 1 {
		fmt.Println(txt)
	}
}
