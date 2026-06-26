package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

// houseStyle returns a go-pretty table style matching the house ASCII format.
func houseStyle() table.Style {
	return table.Style{
		Name: "House ASCII",
		Box: table.BoxStyle{
			BottomLeft:       "+",
			BottomRight:      "+",
			BottomSeparator:  "+",
			EmptySeparator:   "+",
			Left:             "|",
			LeftSeparator:    "+",
			MiddleHorizontal: "-",
			MiddleSeparator:  "+",
			MiddleVertical:   "|",
			PaddingLeft:      " ",
			PaddingRight:     " ",
			Right:            "|",
			RightSeparator:   "+",
			TopLeft:          "+",
			TopRight:         "+",
			TopSeparator:     "+",
			UnfinishedRow:    " ...",
		},
		Color:      table.ColorOptionsDefault,
		Format:     table.FormatOptionsDefault,
		HTML:       table.DefaultHTMLOptions,
		Options:    table.Options{
			DrawBorder:      true,
			SeparateColumns: true,
			SeparateFooter:  true,
			SeparateHeader:  true,
			SeparateRows:    false,
		},
		Title: table.TitleOptionsDefault,
	}
}

func newTable(w io.Writer, headers ...string) table.Writer {
	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(houseStyle())
	hw := make(table.Row, len(headers))
	for i, h := range headers {
		hw[i] = h
	}
	t.AppendHeader(hw)
	t.SetColumnConfigs([]table.ColumnConfig{
		{Number: 2, Align: text.AlignRight, AlignHeader: text.AlignRight},
		{Number: 3, Align: text.AlignRight, AlignHeader: text.AlignRight},
		{Number: 4, Align: text.AlignRight, AlignHeader: text.AlignRight},
		{Number: 5, Align: text.AlignRight, AlignHeader: text.AlignRight},
		{Number: 6, Align: text.AlignRight, AlignHeader: text.AlignRight},
	})
	return t
}

func fmtNum(n int64) string {
	// Format with commas: 12345678 → 12,345,678.
	s := fmt.Sprintf("%d", n)
	var out strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out.WriteByte(',')
		}
		out.WriteRune(c)
	}
	return out.String()
}

func fmtCost(c float64, hasCost bool) string {
	if !hasCost {
		return "n/a"
	}
	return fmt.Sprintf("$%.2f", c)
}

// JSON prints the results as indented JSON.
func JSON(w io.Writer, res *Results) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(res)
}

// AgentTable writes the per-agent table.
func AgentTable(w io.Writer, res *Results) {
	t := newTable(w, "Agent", "Input", "Output", "Cache", "Reasoning", "Cost USD")
	var totalIn, totalOut, totalCache, totalReason int64
	var totalCost float64
	for _, a := range res.Agents {
		cache := a.CacheReadTokens + a.CacheWriteTokens
		t.AppendRow(table.Row{
			a.Agent,
			fmtNum(a.InputTokens),
			fmtNum(a.OutputTokens),
			fmtNum(cache),
			fmtNum(a.ReasoningTokens),
			fmtCost(a.Cost, a.HasCost),
		})
		totalIn += a.InputTokens
		totalOut += a.OutputTokens
		totalCache += cache
		totalReason += a.ReasoningTokens
		if a.HasCost {
			totalCost += a.Cost
		}
	}
	t.AppendSeparator()
	t.AppendRow(table.Row{"TOTAL", fmtNum(totalIn), fmtNum(totalOut), fmtNum(totalCache), fmtNum(totalReason), fmt.Sprintf("$%.2f", totalCost)})
	t.Render()
}

// DayTable writes the per-day table.
func DayTable(w io.Writer, res *Results) {
	t := newTable(w, "Day", "Input", "Output", "Cache", "Reasoning", "Cost USD")
	var totalIn, totalOut, totalCache, totalReason int64
	var totalCost float64
	for _, d := range res.Days {
		cache := d.CacheReadTokens + d.CacheWriteTokens
		t.AppendRow(table.Row{
			d.Date,
			fmtNum(d.InputTokens),
			fmtNum(d.OutputTokens),
			fmtNum(cache),
			fmtNum(d.ReasoningTokens),
			fmt.Sprintf("$%.2f", d.Cost),
		})
		totalIn += d.InputTokens
		totalOut += d.OutputTokens
		totalCache += cache
		totalReason += d.ReasoningTokens
		totalCost += d.Cost
	}
	t.AppendSeparator()
	t.AppendRow(table.Row{"TOTAL", fmtNum(totalIn), fmtNum(totalOut), fmtNum(totalCache), fmtNum(totalReason), fmt.Sprintf("$%.2f", totalCost)})
	t.Render()
}

// MonthTable writes the per-month table.
func MonthTable(w io.Writer, res *Results) {
	t := newTable(w, "Month", "Input", "Output", "Cache", "Reasoning", "Cost USD")
	var totalIn, totalOut, totalCache, totalReason int64
	var totalCost float64
	for _, m := range res.Months {
		cache := m.CacheReadTokens + m.CacheWriteTokens
		t.AppendRow(table.Row{
			m.Month,
			fmtNum(m.InputTokens),
			fmtNum(m.OutputTokens),
			fmtNum(cache),
			fmtNum(m.ReasoningTokens),
			fmt.Sprintf("$%.2f", m.Cost),
		})
		totalIn += m.InputTokens
		totalOut += m.OutputTokens
		totalCache += cache
		totalReason += m.ReasoningTokens
		totalCost += m.Cost
	}
	t.AppendSeparator()
	t.AppendRow(table.Row{"TOTAL", fmtNum(totalIn), fmtNum(totalOut), fmtNum(totalCache), fmtNum(totalReason), fmt.Sprintf("$%.2f", totalCost)})
	t.Render()
}

// Footer writes the priced models summary line.
func Footer(w io.Writer, res *Results) {
	_, _ = fmt.Fprintf(w, "priced: %d of %d models", res.Priced, res.Total)
	if len(res.UnpricedModels) > 0 {
		_, _ = fmt.Fprintf(w, " (unpriced: %s)", strings.Join(res.UnpricedModels, ", "))
	}
	_, _ = fmt.Fprintln(w)
}
