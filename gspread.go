package main

import (
	"os/exec"
	"time"

	"github.com/bitcoinfees/feesim/api"
)

func gspreadPutSheet(csv []byte, bin, spreadsheet, worksheet, auth string) error {
	cmd := exec.Command(bin, spreadsheet, worksheet, auth)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	go func() {
		stdin.Write(csv)
		stdin.Close()
	}()

	return cmd.Run()
}

func gspreadMainPlotter(rrdfile string, bin, spreadsheet, auth string) mainPlotter {
	plotMain := func(resnum int) error {
		t := time.Now().Unix()
		p, err := newMainPlot(t, rrdfile, resnum)
		if err != nil {
			return err
		}
		csv, err := p.CSV()
		if err != nil {
			return err
		}
		var worksheet string
		switch resnum {
		case res1:
			worksheet = "1m"
		case res30:
			worksheet = "30m"
		case res180:
			worksheet = "3h"
		case res1440:
			worksheet = "1d"
		default:
			panic("Error should have been returned by newMainPlot.")
		}
		return gspreadPutSheet(csv, bin, spreadsheet, worksheet, auth)
	}
	return plotMain
}

func gspreadPutProfile(p *profilePlot, bin, spreadsheet, auth string) error {
	conf, err := p.CSV("conf")
	if err != nil {
		return err
	}
	txrate, err := p.CSV("txrate")
	if err != nil {
		return err
	}
	caprate, err := p.CSV("caprate")
	if err != nil {
		return err
	}
	mempool, err := p.CSV("mempool")
	if err != nil {
		return err
	}

	errc := make(chan error)
	putAsync := func(csv []byte, worksheet string) {
		errc <- gspreadPutSheet(csv, bin, spreadsheet, worksheet, auth)
	}

	go putAsync(conf, "profile_conf")
	go putAsync(txrate, "profile_txrate")
	go putAsync(caprate, "profile_caprate")
	go putAsync(mempool, "profile_mempool")

	var errGlobal error
	for i := 0; i < 4; i++ {
		if err := <-errc; err != nil {
			errGlobal = err
		}
	}
	return errGlobal
}

func gspreadProfilePlotter(host, port, bin, spreadsheet, auth string) func() error {
	c := api.NewClient(api.Config{Host: host, Port: port, Timeout: 15})
	plotProfile := func() error {
		p, err := newProfilePlot(c)
		if err != nil {
			return err
		}
		return gspreadPutProfile(p, bin, spreadsheet, auth)
	}
	return plotProfile
}
