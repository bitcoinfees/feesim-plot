package main

import (
	"strings"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"log"
	"time"

	//"github.com/bitcoinfees/feesim/api"
	"github.com/ziutek/rrd"
)

type mainPlot struct {
	res, length int64 // in seconds
	rrdfile, cf string
}

func (p mainPlot) Fetch(t int64) (mainPlotResult, error) {
	r := mainPlotResult{data: make([][]float64, 12), names: make([]string, 12)}

	if p.length%p.res != 0 {
		return r, errors.New("res must divide length.")
	}

	end := time.Unix(t-(t%p.res), 0)
	length := time.Duration(p.length) * time.Second
	start := end.Add(-length)
	res := time.Duration(p.res) * time.Second

	f, err := rrd.Fetch(p.rrdfile, p.cf, start, end, res)
	if err != nil {
		return r, err
	}

	n := p.length / p.res
	if int(n) != f.RowCnt-2 {
		panic("Row number mismatch.")
	}
	if len(f.DsNames) != 11 {
		panic("Col number mismatch.")
	}
	for i, name := range f.DsNames {
		r.data[i+1] = make([]float64, n)
		r.names[i+1] = name
		for j := range r.data[i+1] {
			r.data[i+1][j] = f.ValueAt(i, j)
		}
	}
	ti := start.Unix() + p.res
	r.data[0] = make([]float64, n)
	r.names[0] = "time"
	for j := range r.data[0] {
		r.data[0][j] = float64(ti)
		ti += p.res
	}
	return r, nil
}

type mainPlotResult struct {
	data [][]float64
	names []string
}

func (r mainPlotResult) MarshalText() ([]byte, error) {
	buf := new(bytes.Buffer)
	fmt.Fprintln(buf, strings.Join(r.names, ","))
	for i, t := range r.data[0] {
		fmt.Fprintf(buf, "%d,%d,%d,%d,%d,%d,%d,%d,%d,%d,%f,%f\n",
			int(t),
			int(r.data[1][i]),
			int(r.data[2][i]),
			int(r.data[3][i]),
			int(r.data[4][i]),
			int(r.data[5][i]),
			int(r.data[6][i]),
			int(r.data[7][i]),
			int(r.data[8][i]),
			int(r.data[9][i]),
			r.data[10][i],
			r.data[11][i])
	}
	return buf.Bytes(), nil
}

func main() {
	var rrdfile string
	flag.StringVar(&rrdfile, "f", "", "Path to RRD file.")
	flag.Parse()
	if rrdfile == "" {
		log.Fatal("Must specify RRD file with -f.")
	}

	p := mainPlot{
		res:     60*30,
		length:  3600*3,
		rrdfile: rrdfile,
		cf:      "AVERAGE",
	}
	r, err := p.Fetch(time.Now().Unix())
	if err != nil {
		log.Fatal(err)
	}
	s, _ := r.MarshalText()
	fmt.Printf(string(s))

}
