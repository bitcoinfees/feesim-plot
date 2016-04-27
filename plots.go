package main

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bitcoinfees/feesim/api"
	"github.com/ziutek/rrd"
)

type profilePlot struct {
	txrate_x []float64
	txrate_y []float64

	caprate_x []float64
	caprate_y []float64

	mempool_x []float64
	mempool_y []float64

	conf_x []int
	conf_y []int
}

func (p *profilePlot) Fetch(c *api.Client) error {
	txrate, err := c.TxRate(50)
	if err != nil {
		return err
	}
	p.txrate_x = txrate["x"]
	p.txrate_y = txrate["y"]

	caprate, err := c.CapRate(50)
	if err != nil {
		return err
	}
	p.caprate_x = caprate["x"]
	p.caprate_y = caprate["y"]

	mempool, err := c.MempoolSize(50)
	if err != nil {
		return err
	}
	p.mempool_x = mempool["x"]
	p.mempool_y = mempool["y"]

	var result []float64
	if r, err := c.EstimateFee(0); err != nil {
		return err
	} else {
		result = r.([]float64)
	}
	// De-duplicate result feerates
	conftimes := make(map[int]int)
	for i, feerate := range result {
		feerate := int(feerate)
		conftime := i + 1
		conftimeOld := conftimes[feerate]
		if conftimeOld == 0 || conftime < conftimeOld {
			conftimes[feerate] = conftime
		}
	}
	x := make([]int, 0, len(conftimes))
	y := make([]int, 0, len(conftimes))
	for feerate := range conftimes {
		x = append(x, feerate)
	}
	sort.Ints(x)
	for i, feerate := range x {
		y[i] = conftimes[feerate]
	}
	p.conf_x = x
	p.conf_y = y

	return nil
}

func (p *profilePlot) CSV(subplot string) ([]byte, error) {
	var x, y []float64
	switch subplot {
	case "txrate":
		x, y = p.txrate_x, p.txrate_y
	case "caprate":
		x, y = p.caprate_x, p.caprate_y
	case "mempool":
		x, y = p.mempool_x, p.mempool_y
	case "conf":
		x := make([]float64, len(p.conf_x))
		y := make([]float64, len(p.conf_y))
		for i := range x {
			x[i], y[i] = float64(p.conf_x[i]), float64(p.conf_y[i])
		}
	default:
		return nil, errors.New("Invalid subplot.")
	}
	if x == nil || y == nil {
		return nil, errors.New("Data not yet fetched.")
	}

	buf := new(bytes.Buffer)
	fmt.Fprintln(buf, "x,y")
	for i := range x {
		fmt.Fprintf(buf, "%.0f,%f\n", x[i], y[i])
	}
	return buf.Bytes(), nil
}

type mainPlot struct {
	res, length int64 // in seconds
	rrdfile, cf string

	data  [][]float64
	names []string
}

func (p *mainPlot) Fetch(t int64) error {
	if p.length%p.res != 0 {
		return errors.New("res must divide length.")
	}

	end := time.Unix(t-(t%p.res), 0)
	length := time.Duration(p.length) * time.Second
	start := end.Add(-length)
	res := time.Duration(p.res) * time.Second

	f, err := rrd.Fetch(p.rrdfile, p.cf, start, end, res)
	if err != nil {
		return err
	}

	n := p.length / p.res
	if int(n) != f.RowCnt-2 {
		return errors.New("Row number mismatch.")
	}
	if len(f.DsNames) != 11 {
		return errors.New("Col number mismatch.")
	}

	p.data = make([][]float64, n)
	ti := start.Unix() + p.res
	for i := range p.data {
		p.data[i] = make([]float64, 12)
		for j := range p.data[i] {
			if j == 0 {
				p.data[i][j] = float64(ti)
				ti += p.res
				continue
			}
			p.data[i][j] = f.ValueAt(j-1, i)
		}
	}
	p.names = append([]string{"time"}, f.DsNames...)
	return nil
}

func (p *mainPlot) CSV() ([]byte, error) {
	if p.data == nil {
		return nil, errors.New("Data not yet fetched.")
	}
	buf := new(bytes.Buffer)
	fmt.Fprintln(buf, strings.Join(p.names, ","))
	for _, row := range p.data {
		irow := make([]interface{}, len(row))
		for i, el := range row {
			irow[i] = el
		}
		fmt.Fprintf(buf, "%.0f,%.0f,%.0f,%.0f,%.0f,%.0f,%.0f,%.0f,%.0f,%.0f,%f,%f\n", irow...)
	}
	return buf.Bytes(), nil
}

func newMainPlot(t int64, rrdfile string, resnum int) (*mainPlot, error) {
	plot := new(mainPlot)
	plot.cf = "AVERAGE"
	plot.rrdfile = rrdfile
	switch resnum {
	case res1:
		plot.res = 60
		plot.length = 10800
	case res30:
		plot.res = 1800
		plot.length = 172800
	case res180:
		plot.res = 10800
		plot.length = 1209600
	case res1440:
		plot.res = 86400
		plot.length = 15552000
	default:
		return nil, errors.New("invalid resnum.")
	}
	if err := plot.Fetch(t); err != nil {
		return nil, err
	}
	return plot, nil
}
