/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package main

import (
	"os"

	"github.com/kullanici0606/cmdtools/v2/cmd"
)

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "xargs" {
		// since xargs calls programs that also themselves have command line flags,
		// we do not register any flag here, instead we will manually handle our flags
		cmd.ExecuteXargs()
		return
	}

	cmd.Execute()
}
