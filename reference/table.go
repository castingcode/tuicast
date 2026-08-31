package reference

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type orderRecord struct {
	order    string
	customer string
	status   string
	quantity int
	location string
	item     string
}

type tableModel struct {
	width     int
	height    int
	table     table.Model
	records   []orderRecord
	filter    textinput.Model
	filtering bool
	detail    bool
	sortField int
	columns   int
}

func newTableModel(width, height int) tableModel {
	filter := textinput.New()
	filter.Prompt = "Filter: "
	filter.Placeholder = "order, customer, status, or location"
	filter.CharLimit = 30
	model := tableModel{
		width:   width,
		height:  height,
		records: referenceOrders(),
		filter:  filter,
		table: table.New(
			table.WithFocused(true),
		),
	}
	styles := table.DefaultStyles()
	styles.Header = styles.Header.Bold(true).Foreground(lipgloss.Color("6"))
	styles.Selected = styles.Selected.Bold(true).Reverse(true).Foreground(lipgloss.NoColor{})
	model.table.SetStyles(styles)
	model.resize(width, height)
	return model
}

func (m *tableModel) update(key tea.KeyPressMsg) bool {
	if m.detail {
		switch key.String() {
		case "enter", "esc", "f2":
			m.detail = false
		}
		return false
	}
	if m.filtering {
		switch key.String() {
		case "enter":
			m.filtering = false
			m.filter.Blur()
			return false
		case "esc":
			m.filter.SetValue("")
			m.filtering = false
			m.filter.Blur()
			m.applyRows()
			return false
		}
		var command tea.Cmd
		m.filter, command = m.filter.Update(key)
		_ = command
		m.applyRows()
		return false
	}

	switch key.String() {
	case "esc", "f2":
		return true
	case "enter":
		if len(m.table.SelectedRow()) > 0 {
			m.detail = true
		}
		return false
	case "/":
		m.filtering = true
		m.filter.Focus()
		return false
	case "s", "S":
		m.sortField = (m.sortField + 1) % 3
		m.applyRows()
		return false
	}

	updated, _ := m.table.Update(key)
	m.table = updated
	return false
}

func (m *tableModel) updatePaste(message tea.PasteMsg) tea.Cmd {
	if !m.filtering || m.detail {
		return nil
	}
	var command tea.Cmd
	m.filter, command = m.filter.Update(message)
	m.applyRows()
	return command
}

func (m *tableModel) resize(width, height int) {
	m.width = width
	m.height = height
	m.filter.SetWidth(max(10, min(40, width-8)))
	selectedOrder := ""
	if row := m.table.SelectedRow(); len(row) > 0 {
		selectedOrder = row[0]
	}
	// Bubbles rebuilds its viewport immediately when columns change. Remove
	// rows from the previous schema first so a width transition cannot render
	// a five-cell row against three columns, or the reverse.
	m.table.SetRows(nil)

	switch {
	case width >= 76:
		m.columns = 5
		m.table.SetColumns([]table.Column{
			{Title: "Order", Width: 13},
			{Title: "Customer", Width: 18},
			{Title: "Status", Width: 11},
			{Title: "Qty", Width: 6},
			{Title: "Location", Width: 10},
		})
	case width >= 52:
		m.columns = 4
		m.table.SetColumns([]table.Column{
			{Title: "Order", Width: 13},
			{Title: "Status", Width: 11},
			{Title: "Qty", Width: 6},
			{Title: "Location", Width: 10},
		})
	default:
		m.columns = 3
		m.table.SetColumns([]table.Column{
			{Title: "Order", Width: max(10, width-24)},
			{Title: "Status", Width: 10},
			{Title: "Qty", Width: 5},
		})
	}
	m.table.SetWidth(width)
	m.table.SetHeight(max(4, height-9))
	m.applyRows()
	for index, row := range m.table.Rows() {
		if row[0] == selectedOrder {
			m.table.SetCursor(index)
			break
		}
	}
}

func (m *tableModel) applyRows() {
	records := append([]orderRecord(nil), m.records...)
	sort.SliceStable(records, func(left, right int) bool {
		switch m.sortField {
		case 1:
			if records[left].status != records[right].status {
				return records[left].status < records[right].status
			}
		case 2:
			if records[left].quantity != records[right].quantity {
				return records[left].quantity < records[right].quantity
			}
		}
		return records[left].order < records[right].order
	})

	query := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	rows := make([]table.Row, 0, len(records))
	for _, record := range records {
		searchable := strings.ToLower(fmt.Sprintf("%s %s %s %s %s", record.order, record.customer, record.status, record.location, record.item))
		if query != "" && !strings.Contains(searchable, query) {
			continue
		}
		switch m.columns {
		case 5:
			rows = append(rows, table.Row{record.order, record.customer, record.status, strconv.Itoa(record.quantity), record.location})
		case 4:
			rows = append(rows, table.Row{record.order, record.status, strconv.Itoa(record.quantity), record.location})
		default:
			rows = append(rows, table.Row{record.order, record.status, strconv.Itoa(record.quantity)})
		}
	}
	m.table.SetRows(rows)
	if len(rows) > 0 && m.table.Cursor() < 0 {
		m.table.SetCursor(0)
	} else if m.table.Cursor() >= len(rows) {
		m.table.SetCursor(max(0, len(rows)-1))
	}
}

func (m tableModel) view() string {
	if m.detail {
		record, found := m.selectedRecord()
		if !found {
			return headingStyle.Render("ORDER DETAILS") + "\n\nNo order selected\n\n" + helpStyle.Render("Esc Return")
		}
		return strings.Join([]string{
			headingStyle.Render("ORDER DETAILS"),
			"",
			"Order:     " + record.order,
			"Customer:  " + record.customer,
			"Status:    " + record.status,
			fmt.Sprintf("Quantity:  %d", record.quantity),
			"Location:  " + record.location,
			"Item:      " + record.item,
			"",
			helpStyle.Render("Enter/Esc/F2 Return to table"),
		}, "\n")
	}

	sortName := []string{"Order", "Status", "Quantity"}[m.sortField]
	lines := []string{
		headingStyle.Render("WAREHOUSE ORDERS"),
		"Sort: " + sortName,
	}
	if m.filtering || m.filter.Value() != "" {
		lines = append(lines, m.filter.View())
	}
	lines = append(lines, m.table.View())
	if len(m.table.Rows()) == 0 {
		lines = append(lines, errorStyle.Render("No matching orders"))
	}
	lines = append(lines, helpStyle.Render("Arrows/PgUp/PgDn/Home/End Navigate  Enter Details  / Filter  S Sort  Esc/F2 Menu"))
	return lipgloss.NewStyle().MaxWidth(m.width).Render(strings.Join(lines, "\n"))
}

func (m tableModel) selectedRecord() (orderRecord, bool) {
	row := m.table.SelectedRow()
	if len(row) == 0 {
		return orderRecord{}, false
	}
	for _, record := range m.records {
		if record.order == row[0] {
			return record, true
		}
	}
	return orderRecord{}, false
}

func referenceOrders() []orderRecord {
	customers := []string{"ACME", "Acme Corp", "Widget Inc", "Northwind", "Contoso", "Adventure Works"}
	statuses := []string{"ALLOCATED", "PICKING", "SHIPPED", "HOLD"}
	items := []string{"WIDGET-42", "CAST-ROD-8", "REEL-2500", "LINE-6WT", "FLY-ADAMS", "PACK-DRY"}
	records := make([]orderRecord, 30)
	for index := range records {
		records[index] = orderRecord{
			order:    fmt.Sprintf("ORD-%08d", 10002341+index),
			customer: customers[index%len(customers)],
			status:   statuses[index%len(statuses)],
			quantity: 6 + (index*17)%94,
			location: fmt.Sprintf("%c-%02d-%02d", 'A'+rune(index%4), 1+(index/4)%8, 1+index%12),
			item:     items[index%len(items)],
		}
	}
	return records
}
