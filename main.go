package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
)

const (
	res1 = iota
	res30
	res180
	res1440
)

type mainPlotter func(resnum int) error
type profilePlotter func() error
type miningPlotter func() error

const usage = `
feesim-plot [global options] COMMAND [args...]

Commands:
	main RESNUMBER

`

func main() {
	var (
		rrdfile     string
		bin         string
		spreadsheet string
		auth        string
		logfile     string
	)
	flag.StringVar(&rrdfile, "f", "", "Path to RRD file.")
	flag.StringVar(&bin, "b", "", "path to putsheet binary")
	flag.StringVar(&spreadsheet, "s", "", "spreadsheet name")
	flag.StringVar(&auth, "a", "", "path to gspread json auth token")
	flag.StringVar(&logfile, "l", "", "path to logfile")
	flag.Parse()
	if rrdfile == "" || bin == "" || spreadsheet == "" || auth == "" {
		fmt.Fprintf(os.Stderr, usage)
		flag.CommandLine.PrintDefaults()
		log.Fatal("Insufficient arguments.")
	}

	var logger *log.Logger
	if logfile == "" {
		logger = log.New(os.Stderr, "", log.LstdFlags)
	} else {
		logFileMode := os.O_WRONLY | os.O_CREATE | os.O_APPEND
		if f, err := os.OpenFile(logfile, logFileMode, 0666); err != nil {
			log.Fatal(err)
		} else {
			logger = log.New(f, "", log.LstdFlags)
		}
	}

	plotMain := gspreadMainPlotter(rrdfile, bin, spreadsheet, auth)

	switch flag.Arg(0) {
	case "main":
		if flag.Arg(1) == "" {
			logger.Fatal("Invalid res number.")
		}
		resnum, err := strconv.Atoi(flag.Arg(1))
		if err != nil {
			logger.Fatal("Invalid res num:", err)
		}
		if err := plotMain(resnum); err != nil {
			logger.Fatal("plotMain error:", err)
		}
	default:
		logger.Fatal("Invalid command.")
	}
}
