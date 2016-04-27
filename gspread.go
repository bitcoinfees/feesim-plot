package main

import (
	"os/exec"
	"time"
)

func putGSpread(csv []byte, bin, spreadsheet, worksheet, auth string) error {
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
		return putGSpread(csv, bin, spreadsheet, worksheet, auth)
	}
	return plotMain
}
