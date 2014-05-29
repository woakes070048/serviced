package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
)

var (
	treeCharset map[string]string
	treeUTF8    map[string]string
	treeASCII   map[string]string
)

func init() {
	treeUTF8 = map[string]string{
		"bar":    "│ ",
		"middle": "├─",
		"last":   "└─",
	}

	treeASCII = map[string]string{
		"bar":    "| ",
		"middle": "|-",
		"last":   "+-",
	}

	treeCharset = treeUTF8
}

// treemap is a list of node ids mapped to its respective parent
type treemap map[string][]string

// sort organizes a treemap by the number of child nodes
func (t treemap) sort() {
	for branch := range t {
		sort.Sort(&leaf{t, branch})
	}
}

//leaf is a child node of a tree map, identified by its parent node
type leaf struct {
	tmap   treemap
	branch string
}

// Len implements sort.Interface
func (l *leaf) Len() int { return len(l.tmap[l.branch]) }

// Less implements sort.Interface
func (l *leaf) Less(i, j int) bool { return len(l.tmap[l.branch][i]) < len(l.tmap[l.branch][j]) }

// Swap implements sort.Interface
func (l *leaf) Swap(i, j int) {
	l.tmap[l.branch][i], l.tmap[l.branch][j] = l.tmap[l.branch][j], l.tmap[l.branch][i]
}

// table is the ascii table formatter
type table struct {
	writer    *tabwriter.Writer
	paragraph []string
	lastrow   bool
}

// newtable instantiates a new table formatter
func newtable(minwidth, tabwidth, padding int) *table {
	w := tabwriter.NewWriter(os.Stdout, minwidth, tabwidth, padding, '\t', 0)
	return &table{writer: w}
}

// printrow prints the row to the writer
func (tbl *table) printrow(row ...interface{}) {
	var rowstr = make([]string, len(row))
	for i, c := range row {
		rowstr[i] = fmt.Sprintf("%v", c)
	}
	fmt.Fprintln(tbl.writer, strings.Join(rowstr, "\t"))
}

// printtreerow prints a tree row to the writer
func (tbl *table) printtreerow(row ...interface{}) {
	var charset string
	if tbl.lastrow {
		charset = treeCharset["last"]
	} else {
		charset = treeCharset["middle"]
	}

	row[0] = fmt.Sprintf("%s%s%v", strings.Join(tbl.paragraph, ""), charset, row[0])
	tbl.printrow(row...)
}

// indent adds an indentation to a tree row
func (tbl *table) indent() {
	if tbl.lastrow {
		tbl.paragraph = append(tbl.paragraph, "  ")
	} else {
		tbl.paragraph = append(tbl.paragraph, treeCharset["bar"])
	}
}

// dedent removes an indentation from a tree row
func (tbl *table) dedent() {
	tbl.paragraph = tbl.paragraph[:len(tbl.paragraph)-1]
}

// formattree formats the tree for printing
func (tbl *table) formattree(tree map[string][]string, root string, getrow func(string) []interface{}) {
	tmap := treemap(tree)

	var next func(string)
	next = func(root string) {
		for i, node := range tmap[root] {
			tbl.lastrow = i+1 >= len(tmap[root])
			tbl.printtreerow(getrow(node)...)
			tbl.indent()
			next(node)
			tbl.dedent()
		}
	}
	next(root)
}

// flush flushes the output
func (tbl *table) flush() { tbl.writer.Flush() }