package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"time"

	"github.com/bitcoinfees/feesim/api"
)

func gspreadPutSheet(csv []byte, bin, spreadsheet, worksheet, auth string) (err error) {
	const numtries = 3
	var (
		stdin  io.WriteCloser
		stderr io.ReadCloser
	)

	for i := 0; i < numtries; i++ {
		cmd := exec.Command(bin, spreadsheet, worksheet, auth)
		stdin, err = cmd.StdinPipe()
		if err != nil {
			continue
		}
		stderr, err = cmd.StderrPipe()
		if err != nil {
			continue
		}

		go func() {
			stdin.Write(csv)
			stdin.Close()
		}()

		sec := make(chan []byte)
		go func() {
			b, _ := ioutil.ReadAll(stderr)
			sec <- b
		}()

		errc := make(chan error)
		go func() { errc <- cmd.Run() }()

		toSigInt := time.NewTimer(time.Minute * 2)
		toSigKill := time.NewTimer(time.Minute * 3)

	WaitErr:
		select {
		case err = <-errc:
			toSigInt.Stop()
			toSigKill.Stop()
			se := <-sec
			if err == nil {
				return
			}
			err = fmt.Errorf("%v: %s", err, string(se))
		case <-toSigInt.C:
			cmd.Process.Signal(os.Interrupt)
			goto WaitErr
		case <-toSigKill.C:
			cmd.Process.Signal(os.Kill)
			goto WaitErr
		}
	}
	return
}

func gspreadMainPlotter(rrdfile, bin, spreadsheet, auth string) mainPlotter {
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

	timestr := []byte(fmt.Sprintf("timestr\n%s\n", time.Now().UTC().Format(time.RFC822)))

	go putAsync(conf, "profile_conf")
	go putAsync(txrate, "profile_txrate")
	go putAsync(caprate, "profile_caprate")
	go putAsync(mempool, "profile_mempool")
	go putAsync(timestr, "profile_time")

	var errGlobal error
	for i := 0; i < 5; i++ {
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

func gspreadPutMining(p *miningPlot, bin, spreadsheet, auth string) error {
	mfr, err := p.CSV("mfr")
	if err != nil {
		return err
	}
	mbs, err := p.CSV("mbs")
	if err != nil {
		return err
	}

	errc := make(chan error)
	putAsync := func(csv []byte, worksheet string) {
		errc <- gspreadPutSheet(csv, bin, spreadsheet, worksheet, auth)
	}

	timestr := []byte(fmt.Sprintf("timestr\n%s\n", time.Now().UTC().Format(time.RFC822)))

	go putAsync(mfr, "mining_mfr")
	go putAsync(mbs, "mining_mbs")
	go putAsync(timestr, "mining_time")

	var errGlobal error
	for i := 0; i < 3; i++ {
		if err := <-errc; err != nil {
			errGlobal = err
		}
	}
	return errGlobal
}

func gspreadMiningPlotter(mfrCutoffProb float64, host, port, bin, spreadsheet, auth string) func() error {
	c := api.NewClient(api.Config{Host: host, Port: port, Timeout: 15})
	plotMining := func() error {
		p, err := newMiningPlot(c, mfrCutoffProb)
		if err != nil {
			return err
		}
		return gspreadPutMining(p, bin, spreadsheet, auth)
	}
	return plotMining
}

func gspreadPutScores(p *scoresPlot, bin, spreadsheet, auth string) error {
	s, err := p.CSV()
	if err != nil {
		return err
	}

	errc := make(chan error)
	putAsync := func(csv []byte, worksheet string) {
		errc <- gspreadPutSheet(csv, bin, spreadsheet, worksheet, auth)
	}

	timestr := []byte(fmt.Sprintf("timestr\n%s\n", time.Now().UTC().Format(time.RFC822)))
	go putAsync(s, "predictscores")
	go putAsync(timestr, "predictscores_time")

	var errGlobal error
	for i := 0; i < 2; i++ {
		if err := <-errc; err != nil {
			errGlobal = err
		}
	}
	return errGlobal
}

func gspreadScoresPlotter(host, port, bin, spreadsheet, auth string) func() error {
	c := api.NewClient(api.Config{Host: host, Port: port, Timeout: 15})
	plotScores := func() error {
		p, err := newScoresPlot(c)
		if err != nil {
			return err
		}
		return gspreadPutScores(p, bin, spreadsheet, auth)
	}
	return plotScores
}
