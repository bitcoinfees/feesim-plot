package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
)

const (
	res1 = iota
	res30
	res180
	res1440
)

const coin = 100000000

type mainPlotter func(resnum int) error

const usage = `
feesim-plot [global options] COMMAND [args...]

Commands:
	main -f RRDFILE -n RESNUMBER
	profile [-host HOST] [-port PORT]
	mining [-host HOST] [-port PORT]

`

func doMain(args []string, bin, spreadsheet, auth string) error {
	var (
		rrdfile   string
		resnumber int
	)
	f := flag.NewFlagSet(args[0], flag.ExitOnError)
	f.StringVar(&rrdfile, "f", "", "Path to RRD file.")
	f.IntVar(&resnumber, "n", -1, "Res number, 0-3")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	if rrdfile == "" || resnumber == -1 {
		return errors.New("Insufficient args.")
	}
	plotMain := gspreadMainPlotter(rrdfile, bin, spreadsheet, auth)
	return plotMain(resnumber)
}

func doProfile(args []string, bin, spreadsheet, auth string) error {
	var (
		host, port string
	)
	f := flag.NewFlagSet(args[0], flag.ExitOnError)
	f.StringVar(&host, "host", "localhost", "api host")
	f.StringVar(&port, "port", "8350", "api port")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	plotProfile := gspreadProfilePlotter(host, port, bin, spreadsheet, auth)
	return plotProfile()
}

func doMining(args []string, bin, spreadsheet, auth string) error {
	var (
		host, port    string
		mfrCutoffProb float64
	)
	f := flag.NewFlagSet(args[0], flag.ExitOnError)
	f.StringVar(&host, "host", "localhost", "api host")
	f.StringVar(&port, "port", "8350", "api port")
	f.Float64Var(&mfrCutoffProb, "c", 0.95, "MFR cutoff prob")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	plotMining := gspreadMiningPlotter(mfrCutoffProb, host, port, bin, spreadsheet, auth)
	return plotMining()
}

func main() {
	var (
		bin         string
		spreadsheet string
		auth        string
		logfile     string
	)
	flag.StringVar(&bin, "b", "", "path to putsheet binary")
	flag.StringVar(&spreadsheet, "s", "", "spreadsheet name")
	flag.StringVar(&auth, "a", "", "path to gspread json auth token")
	flag.StringVar(&logfile, "l", "", "path to logfile")
	flag.Parse()
	if bin == "" || spreadsheet == "" || auth == "" {
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

	switch flag.Arg(0) {
	case "main":
		if err := doMain(flag.Args(), bin, spreadsheet, auth); err != nil {
			logger.Fatal(err)
		}
	case "profile":
		if err := doProfile(flag.Args(), bin, spreadsheet, auth); err != nil {
			logger.Fatal(err)
		}
	case "mining":
		if err := doMining(flag.Args(), bin, spreadsheet, auth); err != nil {
			logger.Fatal(err)
		}
	default:
		logger.Fatal("Invalid command.")
	}
}
