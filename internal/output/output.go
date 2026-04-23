package output

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	githubapi "github.com/bariiss/coprem/internal/github"
)

var SortKeys = []string{"date", "gross-amount", "gross-quantity", "key", "net-amount", "net-quantity"}

type Report struct {
	Enterprise string `json:"enterprise"`
	Period     string `json:"period"`
	GroupBy    string `json:"groupBy"`
	Source     any    `json:"source,omitempty"`
	Rows       []Row  `json:"rows"`
}

type TableOptions struct {
	Color bool
}

type Row struct {
	Date             string  `json:"date,omitempty"`
	Key              string  `json:"key"`
	Product          string  `json:"product,omitempty"`
	SKU              string  `json:"sku,omitempty"`
	Model            string  `json:"model,omitempty"`
	User             string  `json:"user,omitempty"`
	Organization     string  `json:"organization,omitempty"`
	CostCenter       string  `json:"costCenter,omitempty"`
	UnitType         string  `json:"unitType,omitempty"`
	PricePerUnit     float64 `json:"pricePerUnit,omitempty"`
	GrossQuantity    float64 `json:"grossQuantity"`
	GrossAmount      float64 `json:"grossAmount"`
	DiscountQuantity float64 `json:"discountQuantity"`
	DiscountAmount   float64 `json:"discountAmount"`
	NetQuantity      float64 `json:"netQuantity"`
	NetAmount        float64 `json:"netAmount"`
}

func RowsFromUsageItems(date string, items []githubapi.UsageItem, userFallback string) []Row {
	rows := make([]Row, 0, len(items))
	for _, item := range items {
		rowDate := item.Date
		if rowDate == "" {
			rowDate = date
		}
		row := Row{
			Date:             rowDate,
			Key:              "total",
			Product:          item.Product,
			SKU:              item.SKU,
			Model:            item.Model,
			User:             firstNonEmpty(item.User, item.Username, userFallback),
			Organization:     firstNonEmpty(item.Organization, item.OrganizationName),
			CostCenter:       firstNonEmpty(item.CostCenterID, item.CostCenterName),
			UnitType:         item.UnitType,
			PricePerUnit:     item.PricePerUnit,
			GrossQuantity:    item.GrossQuantity,
			GrossAmount:      item.GrossAmount,
			DiscountQuantity: item.DiscountQuantity,
			DiscountAmount:   item.DiscountAmount,
			NetQuantity:      item.NetQuantity,
			NetAmount:        item.NetAmount,
		}
		rows = append(rows, row)
	}
	return rows
}

func GroupReport(report Report, groupBy string, breakdown string) Report {
	if groupBy == "none" {
		report.GroupBy = groupBy
		for i := range report.Rows {
			report.Rows[i].Key = keyFor(report.Rows[i], "none")
		}
		return report
	}

	grouped := map[string]Row{}
	for _, row := range report.Rows {
		key := keyFor(row, groupBy)
		breakdownKey := ""
		if breakdown == "model" {
			breakdownKey = valueOrUnknown(row.Model)
		}
		aggregateKey := strings.Join([]string{row.Date, key, breakdownKey}, "\x00")
		current := grouped[aggregateKey]
		if current.Key == "" {
			current = Row{
				Date:         row.Date,
				Key:          key,
				Product:      row.Product,
				SKU:          row.SKU,
				Model:        row.Model,
				User:         row.User,
				Organization: row.Organization,
				CostCenter:   row.CostCenter,
				UnitType:     row.UnitType,
				PricePerUnit: row.PricePerUnit,
			}
			if breakdown == "model" {
				current.Model = valueOrUnknown(row.Model)
			} else if groupBy != "model" {
				current.Model = "(all)"
			}
		}
		current.GrossQuantity += row.GrossQuantity
		current.GrossAmount += row.GrossAmount
		current.DiscountQuantity += row.DiscountQuantity
		current.DiscountAmount += row.DiscountAmount
		current.NetQuantity += row.NetQuantity
		current.NetAmount += row.NetAmount
		grouped[aggregateKey] = current
	}

	rows := make([]Row, 0, len(grouped))
	for _, row := range grouped {
		rows = append(rows, row)
	}
	report.GroupBy = groupBy
	if breakdown != "total" {
		report.GroupBy += "/" + breakdown
	}
	report.Rows = rows
	return report
}

func SortRows(rows []Row, sortBy string) {
	sort.SliceStable(rows, func(i, j int) bool {
		a := rows[i]
		b := rows[j]
		if a.Date != b.Date {
			return a.Date < b.Date
		}
		switch sortBy {
		case "key":
			return a.Key < b.Key
		case "date":
			return a.Date < b.Date
		case "gross-amount":
			return a.GrossAmount > b.GrossAmount
		case "gross-quantity":
			return a.GrossQuantity > b.GrossQuantity
		case "net-quantity":
			return a.NetQuantity > b.NetQuantity
		case "net-amount":
			fallthrough
		default:
			return a.NetAmount > b.NetAmount
		}
	})
}

func WriteJSON(w io.Writer, report Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func WriteCSV(w io.Writer, report Report) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()
	if err := cw.Write([]string{
		"date",
		"key",
		"product",
		"sku",
		"model",
		"user",
		"organization",
		"cost_center",
		"unit_type",
		"gross_quantity",
		"gross_amount",
		"discount_quantity",
		"discount_amount",
		"net_quantity",
		"net_amount",
	}); err != nil {
		return err
	}
	for _, row := range report.Rows {
		if err := cw.Write([]string{
			row.Date,
			row.Key,
			row.Product,
			row.SKU,
			row.Model,
			row.User,
			row.Organization,
			row.CostCenter,
			row.UnitType,
			float(row.GrossQuantity),
			float(row.GrossAmount),
			float(row.DiscountQuantity),
			float(row.DiscountAmount),
			float(row.NetQuantity),
			float(row.NetAmount),
		}); err != nil {
			return err
		}
	}
	return cw.Error()
}

func ResolveColor(w io.Writer, mode string) (bool, error) {
	switch mode {
	case "always":
		return true, nil
	case "never":
		return false, nil
	case "auto":
		file, ok := w.(*os.File)
		if !ok {
			return false, nil
		}
		info, err := file.Stat()
		if err != nil {
			return false, err
		}
		return info.Mode()&os.ModeCharDevice != 0 && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb", nil
	default:
		return false, fmt.Errorf("unsupported color mode %q; use auto, always, or never", mode)
	}
}

func WriteTable(w io.Writer, report Report, options TableOptions) error {
	columns := tableColumns(report)
	rows := [][]string{tableHeader(columns)}
	for _, row := range report.Rows {
		rows = append(rows, tableRow(row, columns))
	}
	widths := columnWidths(rows)
	c := colors{enabled: options.Color}

	if _, err := fmt.Fprintf(w, "%s  %s\n", c.dim("Enterprise:"), c.bold(report.Enterprise)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "%s      %s\n", c.dim("Period:"), c.bold(report.Period)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "%s    %s\n\n", c.dim("Group by:"), c.bold(report.GroupBy)); err != nil {
		return err
	}

	border := tableBorder(widths)
	if _, err := fmt.Fprintln(w, c.dim(border)); err != nil {
		return err
	}
	for i, row := range rows {
		if err := writeTableRow(w, row, widths, columns, i == 0, c); err != nil {
			return err
		}
		if i == 0 {
			if _, err := fmt.Fprintln(w, c.dim(border)); err != nil {
				return err
			}
		}
	}
	_, err := fmt.Fprintln(w, c.dim(border))
	return err
}

type tableColumn struct {
	key   string
	label string
}

func tableColumns(report Report) []tableColumn {
	columns := []tableColumn{}
	if !sameDateAsPeriod(report) {
		columns = append(columns, tableColumn{key: "date", label: "DATE"})
	}
	columns = append(columns, tableColumn{key: "key", label: "KEY"})
	if anyValue(report.Rows, func(row Row) string { return row.Model }) {
		columns = append(columns, tableColumn{key: "model", label: "MODEL"})
	}
	if anyNonDuplicateUser(report.Rows) {
		columns = append(columns, tableColumn{key: "user", label: "USER"})
	}
	if anyValue(report.Rows, func(row Row) string { return row.Organization }) {
		columns = append(columns, tableColumn{key: "org", label: "ORG"})
	}
	columns = append(columns,
		tableColumn{key: "grossQuantity", label: "GROSS"},
		tableColumn{key: "grossAmount", label: "GROSS $"},
		tableColumn{key: "netQuantity", label: "NET"},
		tableColumn{key: "netAmount", label: "NET $"},
	)
	return columns
}

func tableHeader(columns []tableColumn) []string {
	header := make([]string, 0, len(columns))
	for _, column := range columns {
		header = append(header, column.label)
	}
	return header
}

func tableRow(row Row, columns []tableColumn) []string {
	values := make([]string, 0, len(columns))
	for _, column := range columns {
		switch column.key {
		case "date":
			values = append(values, row.Date)
		case "key":
			values = append(values, row.Key)
		case "model":
			values = append(values, emptyDash(row.Model))
		case "user":
			values = append(values, emptyDash(row.User))
		case "org":
			values = append(values, emptyDash(row.Organization))
		case "grossQuantity":
			values = append(values, float(row.GrossQuantity))
		case "grossAmount":
			values = append(values, money(row.GrossAmount))
		case "netQuantity":
			values = append(values, float(row.NetQuantity))
		case "netAmount":
			values = append(values, money(row.NetAmount))
		default:
			values = append(values, "")
		}
	}
	return values
}

func sameDateAsPeriod(report Report) bool {
	if len(report.Rows) == 0 {
		return true
	}
	for _, row := range report.Rows {
		if row.Date != report.Period {
			return false
		}
	}
	return true
}

func anyValue(rows []Row, value func(Row) string) bool {
	for _, row := range rows {
		if strings.TrimSpace(value(row)) != "" {
			return true
		}
	}
	return false
}

func anyNonDuplicateUser(rows []Row) bool {
	for _, row := range rows {
		user := strings.TrimSpace(row.User)
		if user != "" && user != strings.TrimSpace(row.Key) {
			return true
		}
	}
	return false
}

func keyFor(row Row, groupBy string) string {
	switch groupBy {
	case "model":
		return valueOrUnknown(row.Model)
	case "user":
		return valueOrUnknown(row.User)
	case "product":
		return valueOrUnknown(row.Product)
	case "organization":
		return valueOrUnknown(row.Organization)
	case "cost-center":
		return valueOrUnknown(row.CostCenter)
	case "none":
		return firstNonEmpty(row.SKU, row.Product, row.Model, "total")
	default:
		return "total"
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func valueOrUnknown(value string) string {
	if strings.TrimSpace(value) == "" {
		return "(unknown)"
	}
	return value
}

func emptyDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func float(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func money(value float64) string {
	return strconv.FormatFloat(value, 'f', 2, 64)
}

func columnWidths(rows [][]string) []int {
	if len(rows) == 0 {
		return nil
	}
	widths := make([]int, len(rows[0]))
	for _, row := range rows {
		for i, value := range row {
			if len(value) > widths[i] {
				widths[i] = len(value)
			}
		}
	}
	return widths
}

func tableBorder(widths []int) string {
	parts := make([]string, 0, len(widths))
	for _, width := range widths {
		parts = append(parts, strings.Repeat("-", width+2))
	}
	return "+" + strings.Join(parts, "+") + "+"
}

func writeTableRow(w io.Writer, row []string, widths []int, columns []tableColumn, header bool, c colors) error {
	if len(row) != len(widths) {
		return errors.New("table row width mismatch")
	}
	values := make([]string, 0, len(row))
	for i, value := range row {
		padded := padRight(value, widths[i])
		if header {
			padded = c.bold(c.cyan(padded))
		} else if columns[i].key == "key" {
			padded = c.cyan(padded)
		} else if columns[i].key == "netAmount" {
			padded = c.green(padded)
		}
		values = append(values, " "+padded+" ")
	}
	_, err := fmt.Fprintln(w, "|"+strings.Join(values, "|")+"|")
	return err
}

func padRight(value string, width int) string {
	if len(value) >= width {
		return value
	}
	return value + strings.Repeat(" ", width-len(value))
}

type colors struct {
	enabled bool
}

func (c colors) wrap(code, value string) string {
	if !c.enabled {
		return value
	}
	return "\033[" + code + "m" + value + "\033[0m"
}

func (c colors) bold(value string) string  { return c.wrap("1", value) }
func (c colors) dim(value string) string   { return c.wrap("2", value) }
func (c colors) cyan(value string) string  { return c.wrap("36", value) }
func (c colors) green(value string) string { return c.wrap("32", value) }
