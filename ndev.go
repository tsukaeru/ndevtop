package main

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	ndevStatPathPattern = "/sys/class/net/*/statistics"
	ndevFieldIndex      = 4
)

const (
	dataTypeRxBytes   = "rx_bytes"
	dataTypeTxBytes   = "tx_bytes"
	dataTypeRxPackets = "rx_packets"
	dataTypeTxPackets = "tx_packets"
)

var dataTypes = []string{
	dataTypeRxBytes,
	dataTypeTxBytes,
	dataTypeRxPackets,
	dataTypeTxPackets,
}

type dataTypeFormatter struct {
	name   string
	format func(d *NdevData, typ string, duration time.Duration) string
}

var dataTypeFormatters = map[string]dataTypeFormatter{
	dataTypeRxBytes:   dataTypeFormatter{name: "RX bytes", format: BitPerSec},
	dataTypeTxBytes:   dataTypeFormatter{name: "TX bytes", format: BitPerSec},
	dataTypeRxPackets: dataTypeFormatter{name: "RX packets", format: PacketPerSec},
	dataTypeTxPackets: dataTypeFormatter{name: "TX packets", format: PacketPerSec},
}

func BitPerSec(d *NdevData, typ string, duration time.Duration) string {
	diff := d.Diff(typ) * 8
	div := uint64(1)
	unit := " "
	switch {
	case diff >= 1000*1000*1000:
		div = 1000 * 1000 * 1000
		unit = "G"
	case diff >= 1000*1000:
		div = 1000 * 1000
		unit = "M"
	case diff >= 1000:
		div = 1000
		unit = "K"
	}
	return fmt.Sprintf("%.1f %sbps", float64(diff)/float64(duration/time.Second)/float64(div), unit)
}

func PacketPerSec(d *NdevData, typ string, duration time.Duration) string {
	diff := d.Diff(typ)
	div := uint64(1)
	unit := " "
	switch {
	case diff >= 1000*1000*1000:
		div = 1000 * 1000 * 1000
		unit = "G"
	case diff >= 1000*1000:
		div = 1000 * 1000
		unit = "M"
	case diff >= 1000:
		div = 1000
		unit = "K"
	}
	return fmt.Sprintf("%.1f %spps", float64(diff)/float64(duration/time.Second)/float64(div), unit)
}

type NdevData struct {
	dev       string
	cur, prev map[string]uint64
}

func NewNdevData(dev string) *NdevData {
	return &NdevData{
		dev:  dev,
		cur:  make(map[string]uint64),
		prev: make(map[string]uint64),
	}
}

func (d *NdevData) Add(typ string, v uint64) {
	d.prev[typ] = d.cur[typ]
	d.cur[typ] = v
}

func (d *NdevData) Diff(typ string) uint64 {
	return d.cur[typ] - d.prev[typ]
}

type NdevDataSet struct {
	cur     []*NdevData
	history map[string]*NdevData
	sorter  string
	asc     bool
}

func NewNdevDataSet() *NdevDataSet {
	return &NdevDataSet{
		cur:     make([]*NdevData, 0),
		history: make(map[string]*NdevData),
		sorter:  dataTypeRxBytes,
	}
}

func (ds *NdevDataSet) Len() int      { return len(ds.cur) }
func (ds *NdevDataSet) Swap(i, j int) { ds.cur[i], ds.cur[j] = ds.cur[j], ds.cur[i] }
func (ds *NdevDataSet) Less(i, j int) bool {
	if ds.sorter == "name" || ds.cur[i].Diff(ds.sorter) == ds.cur[j].Diff(ds.sorter) {
		return ds.cur[i].dev < ds.cur[j].dev
	}
	return ds.cur[i].Diff(ds.sorter) < ds.cur[j].Diff(ds.sorter)
}

func (ds *NdevDataSet) SetSortOrder(sorter string, asc bool) {
	ds.asc = asc
	for _, typ := range append([]string{"name"}, dataTypes...) {
		if typ == sorter {
			ds.sorter = sorter
		}
	}
}

func (ds *NdevDataSet) Sort() {
	if ds.asc {
		sort.Sort(ds)
	} else {
		sort.Sort(sort.Reverse(ds))
	}
}

func (ds *NdevDataSet) CollectData() error {
	paths, err := filepath.Glob(ndevStatPathPattern)
	if err != nil {
		return err
	}

	ds.cur = ds.cur[:0]
	for _, dir := range paths {
		dev := strings.Split(dir, "/")[ndevFieldIndex]
		if _, ok := ds.history[dev]; !ok {
			ds.history[dev] = NewNdevData(dev)
		}

		for _, typ := range dataTypes {
			fpath := filepath.Join(dir, typ)
			b, err := ioutil.ReadFile(fpath)
			if err != nil {
				return err
			}
			v, err := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
			if err != nil {
				return err
			}
			ds.history[dev].Add(typ, v)
		}
		ds.cur = append(ds.cur, ds.history[dev])
	}

	return nil
}

func (ds *NdevDataSet) FormattedTable(filter string, duration time.Duration) [][]string {
	table := make([][]string, 0, 1+len(ds.cur))
	table = append(table, []string{"dev name"})
	for _, typ := range dataTypes {
		table[0] = append(table[0], dataTypeFormatters[typ].name)
	}
	for _, d := range ds.cur {
		if !strings.Contains(d.dev, filter) {
			continue
		}
		r := make([]string, 1+len(dataTypes))
		r[0] = d.dev
		for i, typ := range dataTypes {
			r[i+1] = dataTypeFormatters[typ].format(d, typ, duration)
		}
		table = append(table, r)
	}
	return table
}
