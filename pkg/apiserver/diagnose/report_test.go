package diagnose

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"testing"

	"github.com/jinzhu/gorm"
	. "github.com/pingcap/check"
	_ "net/http/pprof"
)

func TestT(t *testing.T) {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	CustomVerboseFlag = true
	TestingT(t)
}

var _ = Suite(&testReportSuite{})

type testReportSuite struct{}

func (t *testReportSuite) TestReport(c *C) {
	cli, err := gorm.Open("mysql", "root:@tcp(127.0.0.1:4000)/test?charset=utf8&parseTime=True&loc=Local")
	c.Assert(err, IsNil)
	defer cli.Close()

	startTime := "2020-02-27 19:20:23"
	endTime := "2020-02-27 21:20:23"

	tables, errs := GetReportTablesForDisplay(startTime, endTime, cli)
	for _, tbl := range tables {
		printRows(tbl)
	}
	c.Assert(errs, HasLen, 0)
}

func (t *testReportSuite) TestGetTable(c *C) {
	cli, err := gorm.Open("mysql", "root:@tcp(172.16.5.40:4009)/test?charset=utf8&parseTime=True&loc=Local")
	c.Assert(err, IsNil)
	defer cli.Close()

	startTime := "2020-03-03 17:18:00"
	endTime := "2020-03-03 17:21:00"

	var table *TableDef
	table, err = GetTiKVThreadCPUTable(startTime, endTime, cli)
	c.Assert(err, IsNil)
	printRows(table)
}

func (t *testReportSuite) TestGetCompareTable(c *C) {
	cli, err := gorm.Open("mysql", "root:@tcp(172.16.5.40:4009)/test?charset=utf8&parseTime=True&loc=Local")
	//cli, err := gorm.Open("mysql", "root:@tcp(127.0.0.1:4000)/test?charset=utf8&parseTime=True&loc=Local")
	c.Assert(err, IsNil)
	defer cli.Close()

	startTime1 := "2020-03-03 17:08:00"
	endTime1 := "2020-03-03 17:11:00"

	startTime2 := "2020-03-03 17:18:00"
	endTime2 := "2020-03-03 17:21:00"

	var errs []error
	var tables []*TableDef
	tables, errs = GetCompareReportTables(startTime1, endTime1, startTime2, endTime2, cli)
	for _, tbl := range tables {
		printRows(tbl)
	}
	for _, err := range errs {
		c.Assert(err, IsNil)
	}
	c.Assert(errs, HasLen, 0)
}

func (t *testReportSuite) TestCompareTable(c *C) {
	table1 := TableDef{
		Category:       []string{"header"},
		Title:          "test",
		joinColumns:    []int{1},
		compareColumns: []int{2},
		Column:         []string{"c1", "c2", "c3"},
		Rows:           nil,
	}

	cases := []struct {
		rows1 []TableRowDef
		rows2 []TableRowDef
		out   []TableRowDef
	}{
		{
			rows1: nil,
			rows2: nil,
			out:   []TableRowDef{},
		},
		{
			rows1: []TableRowDef{
				{Values: []string{"0", "0", "0"}},
			},
			rows2: nil,
			out: []TableRowDef{
				{Values: []string{"0", "0", "0", "1", "", ""}},
			},
		},
		{
			rows1: []TableRowDef{
				{Values: []string{"0", "0", "0"}},
			},
			rows2: []TableRowDef{
				{Values: []string{"1", "1", "1"}},
			},
			out: []TableRowDef{
				{Values: []string{"0", "0", "0", "1", "", ""}},
				{Values: []string{"", "1", "", "1", "1", "1"}},
			},
		},
		{
			rows1: []TableRowDef{
				{Values: []string{"0", "0", "0"}},
			},
			rows2: []TableRowDef{
				{Values: []string{"1", "0", "0"}},
			},
			out: []TableRowDef{
				{Values: []string{"0", "0", "0", "0", "1", "0"}},
			},
		},
		{
			rows1: []TableRowDef{
				{Values: []string{"0", "0", "0"}},
			},
			rows2: []TableRowDef{
				{Values: []string{"1", "0", "1"}},
			},
			out: []TableRowDef{
				{Values: []string{"0", "0", "0", "1", "1", "1"}},
			},
		},
	}

	for _, cas := range cases {
		t1 := table1
		t2 := table1
		t1.Rows = cas.rows1
		t2.Rows = cas.rows2
		t, err := compareTable(&t1, &t2)
		c.Assert(err, IsNil)
		c.Assert(len(t.Rows), Equals, len(cas.out))
		for i, row := range t.Rows {
			c.Assert(row.Values, DeepEquals, cas.out[i].Values)
			c.Assert(len(row.SubValues), Equals, len(cas.out[i].SubValues))
			for j, subRow := range cas.out[i].SubValues {
				c.Assert(subRow, DeepEquals, row.SubValues[j])
			}
		}
	}
}

func (t *testReportSuite) TestRoundFloatString(c *C) {
	cases := []struct {
		in  string
		out string
	}{
		{"0", "0"},
		{"1", "1"},
		{"0.8", "0.8"},
		{"0.99", "0.99"},
		{"1.12345", "1.12"},
		{"1.1256", "1.13"},
		{"12345678.1256", "12345678.13"},
		{"0.1256", "0.13"},
		{"0.00234", "0.002"},
		{"0.00254", "0.003"},
		{"0.000000056", "0.00000006"},
		{"0.00000000000000054", "0.0000000000000005"},
		{"0.00000000000000056", "0.0000000000000006"},
		{"65.20832000000001", "65.21"},
	}
	for _, cas := range cases {
		result := RoundFloatString(cas.in)
		c.Assert(result, Equals, cas.out)
	}
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
