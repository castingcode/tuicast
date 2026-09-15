package main

import (
	"fmt"
	"html"
	"strings"

	tuicast "github.com/castingcode/tuicast/sdk/go"
)

const (
	captureCellWidth  = 9
	captureCellHeight = 18
	capturePadding    = 12
	defaultForeground = "#d8dee9"
	defaultBackground = "#1e1e2e"
)

var ansiColors = [...]string{
	"#000000", "#cd3131", "#0dbc79", "#e5e510",
	"#2472c8", "#bc3fbc", "#11a8cd", "#e5e5e5",
	"#666666", "#f14c4c", "#23d18b", "#f5f543",
	"#3b8eea", "#d670d6", "#29b8db", "#ffffff",
}

func terminalCaptureHTML(screen tuicast.Screen) []byte {
	width := screen.Width*captureCellWidth + capturePadding*2
	height := screen.Height*captureCellHeight + capturePadding*2
	var capture strings.Builder
	fmt.Fprintf(&capture, `<div class="tuicast-terminal-capture"><div style="margin:0 0 6px;color:#555;font:12px sans-serif">Terminal %dx%d · revision %d</div>`, screen.Width, screen.Height, screen.Revision)
	fmt.Fprintf(&capture, `<svg xmlns="http://www.w3.org/2000/svg" role="img" aria-label="Terminal screen at revision %d" viewBox="0 0 %d %d" width="%d" height="%d" style="max-width:100%%;height:auto;background:%s;border-radius:6px">`, screen.Revision, width, height, width, height, defaultBackground)

	for row := 0; row < screen.Height; row++ {
		for column := 0; column < screen.Width; column++ {
			cell, ok := screen.CellAt(column, row)
			if !ok || cell.Width == 0 {
				continue
			}
			foreground, background := captureColors(cell)
			cellWidth := max(1, cell.Width) * captureCellWidth
			x := capturePadding + column*captureCellWidth
			y := capturePadding + row*captureCellHeight
			if background != defaultBackground {
				fmt.Fprintf(&capture, `<rect x="%d" y="%d" width="%d" height="%d" fill="%s"/>`, x, y, cellWidth, captureCellHeight, background)
			}
			if cell.Text == "" || cell.Text == " " || cell.Attributes.Has(tuicast.Conceal) {
				continue
			}
			weight := "normal"
			if cell.Attributes.Has(tuicast.Bold) {
				weight = "bold"
			}
			decoration := "none"
			if cell.Attributes.Has(tuicast.Underline) {
				decoration = "underline"
			}
			fmt.Fprintf(&capture, `<text x="%d" y="%d" fill="%s" font-family="ui-monospace,SFMono-Regular,Menlo,Consolas,monospace" font-size="14" font-weight="%s" text-decoration="%s">%s</text>`, x, y+14, foreground, weight, decoration, html.EscapeString(cell.Text))
		}
	}

	if screen.Cursor.Visible {
		x := capturePadding + screen.Cursor.Column*captureCellWidth
		y := capturePadding + screen.Cursor.Row*captureCellHeight
		fmt.Fprintf(&capture, `<rect x="%d" y="%d" width="%d" height="%d" fill="none" stroke="#ffffff" stroke-opacity="0.75"/>`, x, y, captureCellWidth, captureCellHeight)
	}
	capture.WriteString(`</svg></div>`)
	return []byte(capture.String())
}

func captureColors(cell tuicast.Cell) (string, string) {
	foreground := captureColor(cell.Foreground, defaultForeground)
	background := captureColor(cell.Background, defaultBackground)
	if cell.Attributes.Has(tuicast.Reverse) {
		foreground, background = background, foreground
	}
	return foreground, background
}

func captureColor(color tuicast.Color, fallback string) string {
	index := int(color)
	if index < 0 {
		return fallback
	}
	if index < len(ansiColors) {
		return ansiColors[index]
	}
	if index >= 16 && index <= 231 {
		index -= 16
		red, green, blue := index/36, index/6%6, index%6
		levels := [...]int{0, 95, 135, 175, 215, 255}
		return fmt.Sprintf("#%02x%02x%02x", levels[red], levels[green], levels[blue])
	}
	if index >= 232 && index <= 255 {
		level := 8 + (index-232)*10
		return fmt.Sprintf("#%02x%02x%02x", level, level, level)
	}
	return fallback
}
