package main

import (
	"fmt"
	"os"

	"github.com/zoetrope/yiq"
)

var version string

func main() {
	content := os.Stdin

	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(version)
		os.Exit(0)
	}
	if len(os.Args) > 1 && os.Args[1] == "--help" {
		fmt.Print(`yiq - interactive commandline YAML processor
Usage: <yaml string> | yiq [options] [initial filter]

    yiq is a tool that allows you to play with yq filters interactively
    acting direcly on a YAML source of your choice, given through STDIN.
    For all the details about which filters you can use to transform your
    YAML string, see https://mikefarah.gitbook.io/yq/

    yiq supports all command line arguments yq supports, plus
     -q         will print the ending filter to STDOUT, instead of
                printing the resulting filtered YAML, the default.
     --help     prints this help message.
      `)
		os.Exit(0)
	}

	initialquery := "."
	outputquery := false
	yqargs := os.Args[1:]
	for i, arg := range os.Args[1:] {
		i = i + 1
		if arg == "-q" {
			outputquery = true
			yqargs = os.Args[1:i]
			yqargs = append(yqargs, os.Args[i+1:]...)
			break
		} else if arg[0] != '-' {
			initialquery = arg
			yqargs = os.Args[1:i]
			yqargs = append(yqargs, os.Args[i+1:]...)
			break
		}
	}
	e := yiq.NewEngine(content, yqargs, initialquery)
	os.Exit(run(e, outputquery))
}

func run(e *yiq.Engine, outputquery bool) int {
	result := e.Run()
	if result.Err != nil {
		return 2
	}
	if outputquery {
		fmt.Printf("%s", result.Qs)
	} else {
		fmt.Printf("%s", result.Content)
	}
	return 0
}
