package main

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
)

const (
	AppName         = "ndevtop"
	DefaultDuration = 3 * time.Second
)

type App struct {
	ev       chan *tcell.EventKey
	err      chan error
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	duration time.Duration
	app      *tview.Application
	flex     *tview.Flex
	header   *tview.TextView
	field    *tview.InputField
	table    *tview.Table
	stat     *NdevDataSet
	filter   string
}

func NewApp() *App {
	ctx, cancel := context.WithCancel(context.Background())

	a := &App{
		ev:       make(chan *tcell.EventKey),
		err:      make(chan error, 1),
		ctx:      ctx,
		cancel:   cancel,
		duration: DefaultDuration,
		app:      tview.NewApplication(),
		flex:     tview.NewFlex().SetDirection(tview.FlexRow),
		header:   tview.NewTextView().SetTextColor(tcell.ColorDefault).SetDynamicColors(true),
		field:    tview.NewInputField().SetFieldTextColor(tcell.ColorDefault).SetFieldBackgroundColor(tcell.ColorDefault).SetLabelColor(tcell.ColorDefault).SetFieldWidth(0),
		table:    tview.NewTable().SetFixed(1, 0),
		stat:     NewNdevDataSet(),
		filter:   "",
	}
	a.flex.AddItem(a.header, 5, 0, false)
	a.flex.AddItem(a.field, 1, 0, false)
	a.flex.AddItem(a.table, 0, 1, true)

	return a
}

func (a *App) clean() {
	a.cancel()
	close(a.ev)
	close(a.err)
}

func (a *App) getDuration() {
	a.wg.Add(1)

	a.field.SetLabel("set interval (secs): ").SetAcceptanceFunc(tview.InputFieldInteger).SetText("")
	a.field.SetDoneFunc(func(key tcell.Key) {
		defer a.wg.Done()

		switch key {
		case tcell.KeyEnter:
			txt := a.field.GetText()
			d, err := strconv.ParseUint(txt, 10, 64)
			if err != nil {
				return
			}
			if d == 0 {
				return
			}
			a.duration = time.Duration(d) * time.Second
		}
		a.field.SetLabel("").SetText("")
	})

	a.app.SetInputCapture(a.inputFieldCapture)
	defer func() {
		a.app.SetInputCapture(a.commandCapture).SetFocus(a.table)
	}()
	a.app.SetFocus(a.field).Draw()

	a.wg.Wait()
}

func (a *App) getFilter() {
	a.wg.Add(1)

	a.field.SetLabel("set name filter: ").SetAcceptanceFunc(tview.InputFieldMaxLength(15)).SetText("")
	a.field.SetDoneFunc(func(key tcell.Key) {
		defer a.wg.Done()

		switch key {
		case tcell.KeyEnter:
			a.filter = a.field.GetText()
		}
		a.field.SetLabel("").SetText("")
	})

	a.app.SetInputCapture(a.inputFieldCapture)
	defer func() {
		a.app.SetInputCapture(a.commandCapture).SetFocus(a.table)
	}()
	a.app.SetFocus(a.field).Draw()

	a.wg.Wait()
}

func (a *App) updateHeader() {
	s := fmt.Sprintf("%s - %s\n", AppName, time.Now().Format(time.RFC3339))
	s += fmt.Sprint("keys: [::b]q[::-]: quit, [::b]d[::-]: set update interval, [::b]f[::-]: set dev name filter\n")
	s += fmt.Sprint("      [::b]h,l,j,k,g,G,Ctrl-F,Ctrl-B[::-]: scroll table, [::b]n,N[::-]: sort by name (asc,desc)\n")
	s += fmt.Sprint("      [::b]r,R[::-]: sort by RX bytes (asc,desc), [::b]t,T[::-]: sort by TX bytes (asc,desc)\n")
	s += fmt.Sprint("      [::b]i,I[::-]: sort by RX packets (asc,desc), [::b]o,O[::-]: sort by TX packets (asc,desc)\n")
	a.header.SetText(s)
}

func (a *App) updateTable() {
	if err := a.stat.CollectData(); err != nil {
		return
	}
	a.stat.Sort()

	tbl := a.stat.FormattedTable(a.filter, a.duration)
	if len(tbl) == 0 {
		return
	}

	a.table.Clear()

	for c, hdr := range tbl[0] {
		cell := tview.NewTableCell(hdr).SetAttributes(tcell.AttrBold)
		if c > 0 {
			cell.SetAlign(tview.AlignRight)
		}
		a.table.SetCell(0, c, cell)
	}

	for r, row := range tbl[1:] {
		for c, d := range row {
			cell := tview.NewTableCell(d).SetTextColor(tcell.ColorDefault)
			if c > 0 {
				cell.SetText(fmt.Sprintf("%11s", d)).SetAlign(tview.AlignRight)
			} else {
				cell.SetText(fmt.Sprintf("%-15s", d))
			}
			a.table.SetCell(r+1, c, cell)
		}
	}
}

func (a *App) watch() {
	timer := time.NewTimer(a.duration)
	defer func() {
		if !timer.Stop() {
			<-timer.C
		}
	}()

	for {
		select {
		case <-a.ctx.Done():
			return
		case ev := <-a.ev:
			if !timer.Stop() {
				<-timer.C
			}

			switch ev.Key() {
			case tcell.KeyRune:
				switch ev.Rune() {
				case 'd':
					a.getDuration()
				case 'f':
					a.getFilter()
				case 'i':
					a.stat.SetSortOrder(dataTypeRxPackets, true)
				case 'I':
					a.stat.SetSortOrder(dataTypeRxPackets, false)
				case 'n':
					a.stat.SetSortOrder("name", true)
				case 'N':
					a.stat.SetSortOrder("name", false)
				case 'o':
					a.stat.SetSortOrder(dataTypeTxPackets, true)
				case 'O':
					a.stat.SetSortOrder(dataTypeTxPackets, false)
				case 'r':
					a.stat.SetSortOrder(dataTypeRxBytes, true)
				case 'R':
					a.stat.SetSortOrder(dataTypeRxBytes, false)
				case 't':
					a.stat.SetSortOrder(dataTypeTxBytes, true)
				case 'T':
					a.stat.SetSortOrder(dataTypeTxBytes, false)
				case 'q':
					a.cancel()
					a.app.Stop()
					return
				}
			}
			a.updateHeader()
			a.updateTable()
			a.app.Draw()

			timer.Reset(a.duration)
		case <-timer.C:
			a.updateHeader()
			a.updateTable()
			a.app.Draw()
			timer.Reset(a.duration)
		}
	}
}

func (a *App) commandCapture(ev *tcell.EventKey) *tcell.EventKey {
	switch ev.Key() {
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'd', 'f', 'i', 'I', 'n', 'N', 'o', 'O', 'q', 'r', 'R', 't', 'T':
			a.ev <- ev
			return nil
		}
	}
	return ev
}

func (a *App) inputFieldCapture(ev *tcell.EventKey) *tcell.EventKey {
	switch ev.Key() {
	case tcell.KeyTab, tcell.KeyBacktab:
		return nil
	}
	return ev
}

func (a *App) Run() error {
	a.app.SetInputCapture(a.commandCapture)
	a.updateHeader()
	a.updateTable()

	go a.watch()
	defer a.clean()

	if err := a.app.SetRoot(a.flex, true).Run(); err != nil {
		return err
	}

	return nil
}
