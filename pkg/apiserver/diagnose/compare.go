package diagnose

import (
	"fmt"
	"github.com/jinzhu/gorm"
	"github.com/pingcap/errors"
	"math"
	"sort"
	"strconv"
	"strings"
)

func GetCompareReportTables(startTime1, endTime1, startTime2, endTime2 string, db *gorm.DB) ([]*TableDef, []error) {
	var errs []error
	tables1, err1 := GetCompareTables(startTime1, endTime1, db)
	errs = append(errs, err1...)
	tables2, err2 := GetCompareTables(startTime2, endTime2, db)
	errs = append(errs, err2...)
	tables, err3 := CompareTables(tables1, tables2)
	errs = append(errs, err3...)
	return tables, errs
}

func printRows(t *TableDef) {
	if t == nil {
		fmt.Println("table is nil")
		return
	}

	if len(t.Rows) == 0 {
		fmt.Println("table rows is 0")
		return
	}

	fieldLen := t.ColumnWidth()
	//fmt.Println(fieldLen)
	printLine := func(values []string, comment string) {
		line := ""
		for i, s := range values {
			for k := len(s); k < fieldLen[i]; k++ {
				s += " "
			}
			if i > 0 {
				line += "    |    "
			}
			line += s
		}
		if len(comment) != 0 {
			line = line + "    |    " + comment
		}
		fmt.Println(line)
	}

	fmt.Println(strings.Join(t.Category, " - "))
	fmt.Println(t.Title)
	fmt.Println(t.CommentEN)
	printLine(t.Column, "")

	for _, row := range t.Rows {
		printLine(row.Values, row.Comment)
		for i := range row.SubValues {
			printLine(row.SubValues[i], "")
		}
	}
	fmt.Println("")
}

func CompareTables(tables1, tables2 []*TableDef) ([]*TableDef, []error) {
	var errs []error
	resultTables := make([]*TableDef, 0, len(tables1))
	for _, tbl1 := range tables1 {
		for _, tbl2 := range tables2 {
			if strings.Join(tbl1.Category, ",") == strings.Join(tbl2.Category, ",") &&
				tbl1.Title == tbl2.Title {
				//printRows(tbl1)
				//printRows(tbl2)
				table, err := compareTable(tbl1, tbl2)
				if err != nil {
					errs = append(errs, err)
				} else if table != nil {
					resultTables = append(resultTables, table)
				}
			}
		}

	}
	return resultTables, errs
}

func compareTable(table1, table2 *TableDef) (*TableDef, error) {
	labelsMap1, err := getTableLablesMap(table1)
	if err != nil {
		return nil, err
	}
	labelsMap2, err := getTableLablesMap(table2)
	if err != nil {
		return nil, err
	}

	resultRows := make([]TableRowDef, 0, len(table1.Rows))
	for _, row1 := range table1.Rows {
		label1 := genRowLabel(row1.Values, table1.joinColumns)
		row2, ok := labelsMap2[label1]
		if !ok {
			row2 = &TableRowDef{}
			//return nil, errors.Errorf("category %v,table %v doesn't find row label: %v", strings.Join(table2.Category, ","), table2.Title, label1)
		}
		newRow, err := joinRow(&row1, row2, table1)
		if err != nil {
			return nil, err
		}
		resultRows = append(resultRows, *newRow)
	}
	for _, row2 := range table2.Rows {
		label2 := genRowLabel(row2.Values, table2.joinColumns)
		row1, ok := labelsMap1[label2]
		if ok {
			continue
			//return nil, errors.Errorf("category %v,table %v doesn't find row label: %v", strings.Join(table2.Category, ","), table2.Title, label1)
		}
		row1 = &TableRowDef{}
		newRow, err := joinRow(row1, &row2, table1)
		if err != nil {
			return nil, err
		}
		resultRows = append(resultRows, *newRow)
	}

	resultTable := &TableDef{
		Category:       table1.Category,
		Title:          table1.Title,
		CommentEN:      table1.CommentEN,
		CommentCN:      table1.CommentCN,
		joinColumns:    nil,
		compareColumns: nil,
	}
	columns := make([]string, 0, len(table1.Column)*2-len(table1.joinColumns))
	for i := range table1.Column {
		columns = append(columns, "t1."+table1.Column[i])
	}
	columns = append(columns, "DIFF_RATIO")
	for i := range table2.Column {
		if !checkIn(i, table2.joinColumns) {
			columns = append(columns, "t2."+table1.Column[i])
		}
	}
	sort.Slice(resultRows, func(i, j int) bool {
		return resultRows[i].ratio > resultRows[j].ratio
	})
	resultTable.Column = columns
	resultTable.Rows = resultRows
	return resultTable, nil
}

func joinRow(row1, row2 *TableRowDef, table *TableDef) (*TableRowDef, error) {
	rowsMap1, err := genRowsLablesMap(table, row1.SubValues)
	if err != nil {
		return nil, err
	}
	rowsMap2, err := genRowsLablesMap(table, row2.SubValues)
	if err != nil {
		return nil, err
	}

	subJoinRows := make([]*newJoinRow, 0, len(row1.SubValues))
	for _, subRow1 := range row1.SubValues {
		label := genRowLabel(subRow1, table.joinColumns)
		subRow2 := rowsMap2[label]
		ratio, err := calculateDiffRatio(subRow1, subRow2, table)
		if err != nil {
			return nil, errors.Errorf("category %v,table %v, calculate diff ratio error: %v,  %v,%v", strings.Join(table.Category, ","), table.Title, err.Error(), subRow1, subRow2)
		}
		//fmt.Printf("%v     %v        %v -------\n", ratio, subRow1, subRow2)
		subJoinRows = append(subJoinRows, &newJoinRow{
			row1:  subRow1,
			row2:  subRow2,
			ratio: ratio,
		})
	}

	for _, subRow2 := range row2.SubValues {
		label := genRowLabel(subRow2, table.joinColumns)
		subRow1, ok := rowsMap1[label]
		if ok {
			continue
		}
		ratio, err := calculateDiffRatio(subRow1, subRow2, table)
		if err != nil {
			return nil, errors.Errorf("category %v,table %v, calculate diff ratio error: %v,  %v,%v", strings.Join(table.Category, ","), table.Title, err.Error(), subRow1, subRow2)
		}

		subJoinRows = append(subJoinRows, &newJoinRow{
			row1:  subRow1,
			row2:  subRow2,
			ratio: ratio,
		})
	}

	sort.Slice(subJoinRows, func(i, j int) bool {
		return subJoinRows[i].ratio > subJoinRows[j].ratio
	})
	totalRatio := float64(0)
	resultSubRows := make([][]string, 0, len(row1.SubValues))
	for _, r := range subJoinRows {
		totalRatio += r.ratio
		resultSubRows = append(resultSubRows, r.genNewRow(table))
	}

	// row join with null row
	if len(subJoinRows) == 0 {
		if len(row1.Values) != len(row2.Values) {
			totalRatio = 1
		} else {
			totalRatio, err = calculateDiffRatio(row1.Values, row2.Values, table)
			if err != nil {
				return nil, errors.Errorf("category %v,table %v, calculate diff ratio error: %v,  %v,%v", strings.Join(table.Category, ","), table.Title, err.Error(), row1.Values, row2.Values)
			}
		}
	}

	resultJoinRow := newJoinRow{
		row1:  row1.Values,
		row2:  row2.Values,
		ratio: totalRatio,
	}

	resultRow := &TableRowDef{
		Values:    resultJoinRow.genNewRow(table),
		SubValues: resultSubRows,
		ratio:     totalRatio,
		Comment:   "",
	}
	return resultRow, nil
}

type newJoinRow struct {
	row1  []string
	row2  []string
	ratio float64
}

func (r *newJoinRow) genNewRow(table *TableDef) []string {
	newRow := make([]string, 0, len(r.row1)+len(r.row2))
	ratio := convertFloatToString(r.ratio)
	if len(r.row1) == 0 {
		newRow = append(newRow, make([]string, len(r.row2))...)
		newRow = append(newRow, ratio)
		for i := range r.row2 {
			if checkIn(i, table.joinColumns) {
				newRow[i] = r.row2[i]
			} else {
				newRow = append(newRow, r.row2[i])
			}
		}
		return newRow
	}

	newRow = append(newRow, r.row1...)
	newRow = append(newRow, ratio)
	if len(r.row2) == 0 {
		newRow = append(newRow, make([]string, len(r.row1)-len(table.joinColumns))...)
		return newRow
	}
	for i := range r.row2 {
		if !checkIn(i, table.joinColumns) {
			newRow = append(newRow, r.row2[i])
		}
	}
	return newRow
}

func calculateDiffRatio(row1, row2 []string, table *TableDef) (float64, error) {
	if len(table.compareColumns) == 0 {
		fmt.Printf("category %v,table %v doesn't specified the compare columns", strings.Join(table.Category, ","), table.Title)
		return 0, nil
	}
	if len(row1) == 0 && len(row2) == 0 {
		return 0, nil
	}
	if len(row1) == 0 || len(row2) == 0 {
		return float64(len(table.compareColumns)), nil
	}
	ratio := float64(0)
	for _, idx := range table.compareColumns {
		f1, err := parseFloat(row1[idx])
		if err != nil {
			return 0, err
		}
		f2, err := parseFloat(row2[idx])
		if err != nil {
			return 0, err
		}
		if f1 == f2 {
			continue
		}
		if f1 == 0 || f2 == 0 {
			ratio += 1
		}
		ratio += math.Abs(f1-f2) / math.Max(f1, f2)
	}
	return ratio, nil
}

func parseFloat(s string) (float64, error) {
	if len(s) == 0 {
		return float64(0), nil
	}
	ratio := float64(1)
	if strings.HasSuffix(s, " MB") {
		ratio = 1024 * 1024
		s = s[:len(s)-3]
	} else if strings.HasSuffix(s, " KB") {
		ratio = 1024
		s = s[:len(s)-3]
	}

	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return f * ratio, nil
}

func checkIn(idx int, idxs []int) bool {
	for _, i := range idxs {
		if i == idx {
			return true
		}
	}
	return false
}

func genRowLabel(row []string, joinColumns []int) string {
	label := ""
	for i, idx := range joinColumns {
		if i > 0 {
			label += ","
		}
		label += row[idx]
	}
	return label
}

func genRowsLablesMap(table *TableDef, rows [][]string) (map[string][]string, error) {
	labelsMap := make(map[string][]string, len(rows))
	for i := range rows {
		label := genRowLabel(rows[i], table.joinColumns)
		_, ok := labelsMap[label]
		if ok {
			return nil, errors.Errorf("category %v,table %v has duplicate join label: %v", strings.Join(table.Category, ","), table.Title, label)
		}
		labelsMap[label] = rows[i]
	}
	return labelsMap, nil
}

func getTableLablesMap(table *TableDef) (map[string]*TableRowDef, error) {
	if len(table.joinColumns) == 0 {
		return nil, errors.Errorf("category %v,table %v doesn't have join columns", strings.Join(table.Category, ","), table.Title)
	}
	labelsMap := make(map[string]*TableRowDef, len(table.Rows))
	for i := range table.Rows {
		label := genRowLabel(table.Rows[i].Values, table.joinColumns)
		_, ok := labelsMap[label]
		if ok {
			return nil, errors.Errorf("category %v,table %v has duplicate join label: %v", strings.Join(table.Category, ","), table.Title, label)
		}
		labelsMap[label] = &table.Rows[i]
	}
	return labelsMap, nil
}

func GetCompareTables(startTime, endTime string, db *gorm.DB) ([]*TableDef, []error) {
	funcs := []func(string, string, *gorm.DB) (*TableDef, error){
		// Node
		GetLoadTable,
		GetCPUUsageTable,
		GetTiKVThreadCPUTable,
		GetGoroutinesCountTable,
		//
		// Overview
		GetTotalTimeConsumeTable,
		GetTotalErrorTable,
		//
		//// TiDB
		GetTiDBTimeConsumeTable,
		GetTiDBTxnTableData,
		GetTiDBDDLOwner,
		//
		//// PD
		//GetPDTimeConsumeTable,
		//GetPDSchedulerInfo,
		//GetPDClusterStatusTable,
		//GetStoreStatusTable,
		//GetPDEtcdStatusTable,
		//
		//// TiKV
		//GetTiKVTotalTimeConsumeTable,
		//GetTiKVErrorTable,
		//GetTiKVStoreInfo,
		//GetTiKVRegionSizeInfo,
		//GetTiKVCopInfo,
		//GetTiKVSchedulerInfo,
		//GetTiKVRaftInfo,
		//GetTiKVSnapshotInfo,
		//GetTiKVGCInfo,
		//GetTiKVTaskInfo,
		//GetTiKVCacheHitTable,
		//
		//// Config
		//GetPDConfigInfo,
		//GetTiDBGCConfigInfo,
	}
	tables := make([]*TableDef, 0, len(funcs))
	errs := make([]error, 0, len(funcs))
	for _, f := range funcs {
		tbl, err := f(startTime, endTime, db)
		if err != nil {
			errs = append(errs, err)
		}
		if tbl != nil {
			tables = append(tables, tbl)
		}
	}
	return tables, errs
}