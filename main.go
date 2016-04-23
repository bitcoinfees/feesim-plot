package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	//"github.com/bitcoinfees/feesim/api"
	"github.com/ziutek/rrd"
)

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

func (p *mainPlot) MarshalText() ([]byte, error) {
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

func main() {
	var rrdfile string
	flag.StringVar(&rrdfile, "f", "", "Path to RRD file.")
	flag.Parse()
	if rrdfile == "" {
		log.Fatal("Must specify RRD file with -f.")
	}

	p := mainPlot{
		res:     60,
		length:  600,
		rrdfile: rrdfile,
		cf:      "AVERAGE",
	}
	if err := p.Fetch(time.Now().Unix()); err != nil {
		log.Fatal(err)
	}
	s, err := p.MarshalText()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf(string(s))
}
