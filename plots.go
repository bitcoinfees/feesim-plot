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

type scoresPlot struct {
	scores  []float64
	txTotal []float64
}

func (p *scoresPlot) Fetch(c *api.Client) error {
	s, err := c.Scores()
	if err != nil {
		return err
	}

	scores := make([]float64, len(s["attained"]))
	txTotal := make([]float64, len(s["attained"]))
	for i := range scores {
		txTotal[i] = s["attained"][i] + s["exceeded"][i]
		scores[i] = s["attained"][i] / txTotal[i]
	}

	p.scores = scores
	p.txTotal = txTotal
	return nil
}

func (p *scoresPlot) CSV() ([]byte, error) {
	if p.scores == nil || p.txTotal == nil {
		return nil, errors.New("Data not yet fetched.")
	}
	buf := new(bytes.Buffer)
	fmt.Fprintln(buf, "conf,scores,txtotal")
	for i, score := range p.scores {
		fmt.Fprintf(buf, "%d,%f,%.0f\n", i+1, score, p.txTotal[i])
	}
	return buf.Bytes(), nil
}

func newScoresPlot(c *api.Client) (*scoresPlot, error) {
	p := new(scoresPlot)
	if err := p.Fetch(c); err != nil {
		return nil, err
	}
	return p, nil
}

type miningPlot struct {
	mfr_x []float64
	mfr_y []float64

	mbs_x []float64
	mbs_y []float64
}

func (p *miningPlot) Fetch(c *api.Client, mfrCutoffProb float64) error {
	blksrc, err := c.BlockSource()
	if err != nil {
		return err
	}

	mfr := blksrc["minfeerates"].([]interface{})
	mfrLen := len(mfr)
	var mfr_x []float64
	for _, feerate := range mfr {
		f := feerate.(float64)
		if f >= 0 {
			// f == -1 means +Inf MFR
			mfr_x = append(mfr_x, f)
		}
	}
	mfr_y := make([]float64, len(mfr_x))
	for i := range mfr_y {
		mfr_y[i] = float64(i+1) / float64(mfrLen)
	}
	if i := sort.SearchFloat64s(mfr_y, mfrCutoffProb); i < len(mfr_y) {
		mfr_x = mfr_x[:i]
		mfr_y = mfr_y[:i]
	}

	mbs := blksrc["maxblocksizes"].([]interface{})
	mbsLen := len(mbs)
	mbs_x := make([]float64, len(mbs))
	for i, size := range mbs {
		mbs_x[i] = size.(float64)
	}
	mbs_y := make([]float64, len(mbs_x))
	for i := range mbs_y {
		mbs_y[i] = float64(i) / float64(mbsLen)
	}

	p.mfr_x = mfr_x
	p.mfr_y = mfr_y
	p.mbs_x = mbs_x
	p.mbs_y = mbs_y
	return nil
}

func (p *miningPlot) CSV(subplot string) ([]byte, error) {
	var x, y []float64
	switch subplot {
	case "mfr":
		x, y = p.mfr_x, p.mfr_y
	case "mbs":
		x, y = p.mbs_x, p.mbs_y
	default:
		return nil, errors.New("Invalid subplot.")
	}
	if x == nil || y == nil {
		return nil, errors.New("Data not yet fetched: " + subplot)
	}

	buf := new(bytes.Buffer)
	fmt.Fprintln(buf, "x,y")
	for i := range x {
		fmt.Fprintf(buf, "%.0f,%f\n", x[i], y[i])
	}
	return buf.Bytes(), nil
}

func newMiningPlot(c *api.Client, mfrCutoffProb float64) (*miningPlot, error) {
	p := new(miningPlot)
	if err := p.Fetch(c, mfrCutoffProb); err != nil {
		return nil, err
	}
	return p, nil
}

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
	txrate, err := c.TxRate(20)
	if err != nil {
		return err
	}
	// Convert to bytes/decaminute
	for i := range txrate["y"] {
		txrate["y"][i] *= 600
	}
	p.txrate_x = txrate["x"]
	p.txrate_y = txrate["y"]

	caprate, err := c.CapRate(50)
	if err != nil {
		return err
	}
	// Convert to bytes/decaminute
	for i := range caprate["y"] {
		caprate["y"][i] *= 600
	}
	p.caprate_x = caprate["x"]
	p.caprate_y = caprate["y"]

	mempool, err := c.MempoolSize(30)
	if err != nil {
		return err
	}
	p.mempool_x = mempool["x"]
	p.mempool_y = mempool["y"]

	var result []float64
	if r, err := c.EstimateFee(0); err != nil {
		return err
	} else {
		rslice := r.([]interface{})
		result = make([]float64, len(rslice))
		for i, feerate := range rslice {
			result[i] = feerate.(float64) * coin
		}
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
	for feerate := range conftimes {
		x = append(x, feerate)
	}
	sort.Ints(x)
	y := make([]int, len(x))
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
		x = make([]float64, len(p.conf_x))
		y = make([]float64, len(p.conf_y))
		for i := range x {
			x[i], y[i] = float64(p.conf_x[i]), float64(p.conf_y[i])
		}
	default:
		return nil, errors.New("Invalid subplot.")
	}
	if x == nil || y == nil {
		return nil, errors.New("Data not yet fetched: " + subplot)
	}

	buf := new(bytes.Buffer)
	fmt.Fprintln(buf, "x,y")
	for i := range x {
		fmt.Fprintf(buf, "%.0f,%f\n", x[i], y[i])
	}
	return buf.Bytes(), nil
}

func newProfilePlot(c *api.Client) (*profilePlot, error) {
	p := new(profilePlot)
	if err := p.Fetch(c); err != nil {
		return nil, err
	}
	return p, nil
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
	// Convert to bytes/decaminute for txbyterate and capbyterate
	p.names = append([]string{"time"}, f.DsNames...)
	for i := range p.data {
		p.data[i][10] *= 600
		p.data[i][11] *= 600
	}
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
