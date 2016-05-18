package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"gopkg.in/yaml.v2"
)

const (
	res1 = iota
	res30
	res180
	res1440
)

const (
	coin    = 100000000
	version = "0.1.0"
)

type mainPlotter func(resnum int) error

const usage = `
feesim-plot [global options] COMMAND [args...]

Commands:
	loop -f RRDFILE [-host HOST] [-port PORT]
	main -f RRDFILE -n RESNUMBER
	profile [-host HOST] [-port PORT]
	mining [-host HOST] [-port PORT]
	predictscores [-host HOST] [-port PORT]

`

type loopConfig struct {
	Name   string `yaml:"name"`
	Period int64  `yaml:"period"`
	Offset int64  `yaml:"offset"`
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

	if flag.Arg(0) == "version" {
		fmt.Println(version)
		os.Exit(0)
	}

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
	case "loop":
		if err := doLoop(flag.Args(), bin, spreadsheet, auth, logger); err != nil {
			logger.Fatal(err)
		}
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
	case "predictscores":
		if err := doScores(flag.Args(), bin, spreadsheet, auth); err != nil {
			logger.Fatal(err)
		}
	default:
		logger.Fatal("Invalid command.")
	}
}

func loop(f func() error, cfg loopConfig, logger *log.Logger, wg *sync.WaitGroup, done <-chan struct{}) {
	defer wg.Done()

	// Wait until unix time (s) is a multiple of loop period,
	// then wait further for offset seconds.
	t := time.Now().Unix()
	wait := cfg.Period - (t % cfg.Period) + cfg.Offset
	waitc := time.After(time.Duration(wait) * time.Second)
	logger.Printf("Starting loop %s, waiting for %ds", cfg.Name, wait)
	select {
	case <-waitc:
	case <-done:
		return
	}
	logger.Printf("Loop %s, wait done, starting main loop", cfg.Name)

	// Loop starts
	ticker := time.NewTicker(time.Duration(cfg.Period) * time.Second)
	defer ticker.Stop()
	for {
		if err := f(); err != nil {
			logger.Printf("[ERROR] Plotting %s: %v", cfg.Name, err)
		} else {
			logger.Printf("Plotted %s", cfg.Name)
		}
		select {
		case <-ticker.C:
		case <-done:
			return
		}
	}
}

func doLoop(args []string, bin, spreadsheet, auth string, logger *log.Logger) error {
	var (
		rrdfile    string
		configfile string
		host, port string
	)
	f := flag.NewFlagSet(args[0], flag.ExitOnError)
	f.StringVar(&rrdfile, "f", "./rrd.db", "Path to RRD file.")
	f.StringVar(&configfile, "c", "./plotcfg.yml", "Path to loop config file.")
	f.StringVar(&host, "host", "localhost", "api host")
	f.StringVar(&port, "port", "8350", "api port")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	if rrdfile == "" {
		return errors.New("Need to specify RRD file with -f.")
	}

	var cfg []loopConfig
	if c, err := ioutil.ReadFile(configfile); err != nil {
		return err
	} else if err := yaml.Unmarshal(c, &cfg); err != nil {
		return err
	}

	wg := new(sync.WaitGroup)
	done := make(chan struct{})
	plotMain := gspreadMainPlotter(rrdfile, bin, spreadsheet, auth)
	for _, c := range cfg {
		var f func() error
		switch c.Name {
		case "1m":
			f = func() error { return plotMain(res1) }
		case "30m":
			f = func() error { return plotMain(res30) }
		case "3h":
			f = func() error { return plotMain(res180) }
		case "1d":
			f = func() error { return plotMain(res1440) }
		case "profile":
			f = gspreadProfilePlotter(host, port, bin, spreadsheet, auth)
		case "mining":
			f = gspreadMiningPlotter(0.95, host, port, bin, spreadsheet, auth)
		case "scores":
			f = gspreadScoresPlotter(host, port, bin, spreadsheet, auth)
		default:
			return fmt.Errorf("Loop config error: invalid plot name %s.", c.Name)
		}
		wg.Add(1)
		go loop(f, c, logger, wg, done)
	}
	logger.Println("Plot loops started.")

	sigc := make(chan os.Signal, 3)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	<-sigc
	close(done)
	logger.Println("Received signal, waiting on goroutines..")
	wg.Wait()
	logger.Println("Shutdown OK")
	return nil
}

func doMain(args []string, bin, spreadsheet, auth string) error {
	var (
		rrdfile   string
		resnumber int
	)
	f := flag.NewFlagSet(args[0], flag.ExitOnError)
	f.StringVar(&rrdfile, "f", "./rrd.db", "Path to RRD file.")
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

func doScores(args []string, bin, spreadsheet, auth string) error {
	var (
		host, port string
	)
	f := flag.NewFlagSet(args[0], flag.ExitOnError)
	f.StringVar(&host, "host", "localhost", "api host")
	f.StringVar(&port, "port", "8350", "api port")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	plotScores := gspreadScoresPlotter(host, port, bin, spreadsheet, auth)
	return plotScores()
}
