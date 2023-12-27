/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

var name string
var mmin string

// findCmd represents the find command
var findCmd = &cobra.Command{
	Use:   "find [path]",
	Short: "Unix find command",
	Run: func(cmd *cobra.Command, args []string) {
		executeFind(args)
	},
}

func init() {
	rootCmd.AddCommand(findCmd)
	findCmd.Flags().StringVar(&name, "name", "", "name pattern")
	findCmd.Flags().StringVar(&mmin, "mmin", "", "file modification diff minutes")
}

type fileFilter interface {
	Accept(string) bool
}

type nameFilter struct {
	name string
}

func (n *nameFilter) Accept(path string) bool {
	fileName := filepath.Base(path)
	matched, err := filepath.Match(n.name, fileName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "file name pattern error", err)
		return false
	}
	return matched
}

type timeFilterType int

const (
	lessThan timeFilterType = iota
	exactly
	moreThan
)

type timeFilter struct {
	threshold  time.Time
	filterType timeFilterType
}

func (t *timeFilter) Accept(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Cannot stat file", err)
		return false
	}
	fileModTime := info.ModTime().Truncate(time.Minute)
	switch t.filterType {
	case exactly:
		return fileModTime.Equal(t.threshold)
	case lessThan:
		return fileModTime.After(t.threshold)
	case moreThan:
		return fileModTime.Before(t.threshold)
	}
	return false
}

func parseMmin(mmin string, now time.Time) (*timeFilter, error) {
	filterType := exactly
	sign := 1
	if mmin[0] == '-' {
		filterType = lessThan
		mmin = mmin[1:]
		sign = -1
	} else if mmin[0] == '+' {
		filterType = moreThan
		mmin = mmin[1:]
	}

	i, err := strconv.Atoi(mmin)
	if err != nil {
		return nil, err
	}

	diff := sign * i * int(time.Minute)
	threshold := now.Add(time.Duration(diff))

	return &timeFilter{
		threshold:  threshold,
		filterType: filterType,
	}, nil

}

func executeFind(args []string) {
	path := "."
	if len(args) != 0 {
		path = args[0]
	}

	filters := make([]fileFilter, 0)

	if len(name) != 0 {
		filters = append(filters, &nameFilter{name: name})
	}

	if len(mmin) != 0 {
		mminFilter, err := parseMmin(mmin, time.Now())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot parse parameter mmin: %s %s\n", mmin, err)
			return
		}
		filters = append(filters, mminFilter)
	}

	findAll(path, filters)
}

func findAll(path string, filters []fileFilter) {
	filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error while traversing %s %s\n", path, err)
			return nil
		}

		matched := true
		for _, filter := range filters {
			matched = matched && filter.Accept(path)
		}

		if matched {
			fmt.Println(path)
		}

		return nil
	})
}
