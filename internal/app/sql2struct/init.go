package sql2struct

import (
	"fmt"
	"fyne.io/fyne"
	"fyne.io/fyne/dialog"
	"fyne.io/fyne/layout"
	"fyne.io/fyne/widget"
	"github.com/liudanking/gorm2sql/program"
	"github.com/xormplus/xorm/schemas"
	"go/ast"
	"go/parser"
	"go/token"
	"gormat/configs"
	"gormat/internal/pkg/icon"
	"gormat/pkg/sql2struct"
	"gormat/pkg/sql2struct/quickly"
	"gormat/pkg/sql2struct/sqlorm"
	"log"
	"path/filepath"
	"strings"
	"time"
)

//Screen the sql2Struct screen
func Screen(win fyne.Window, dbConf *sql2struct.SourceMap) *fyne.Container {
	resultBox := widget.NewMultiLineEntry()
	resultBox.SetPlaceHolder(``)
	progressDialog := dialog.NewProgress(dbConf.Host, dbConf.Db[0], win)
	go func() {
		num := 0.0
		for num <= 1.0 {
			time.Sleep(50 * time.Millisecond)
			progressDialog.SetValue(num)
			num += 0.01
		}
	}()
	defer progressDialog.Hide()
	if err := sql2struct.Init(dbConf); err != nil {
		return fyne.NewContainerWithLayout(
			layout.NewGridLayout(1),
			widget.NewLabel(err.Error()),
		)
	}
	var tables = widget.NewTabContainer()
	var currentTable = make(chan *widget.TabItem)
	if tbs, err := sql2struct.Engine.DBMetas(); err == nil {
		for _, t := range tbs {
			tables.Append(widget.NewTabItemWithIcon(t.Name, icon.Table, widget.NewMultiLineEntry()))
		}
		tables.SelectTabIndex(0)
		go func() {
			for {
				time.Sleep(time.Microsecond * 50)
				if <-currentTable != tables.CurrentTab() {
					var currentT *schemas.Table
					for _, t := range tbs {
						if t.Name == tables.CurrentTab().Text {
							currentT = t
						}
					}
					if currentT == nil {
						return
					}
					tableBox := widget.NewMultiLineEntry()
					if result, err := sql2struct.NewGenTool().Gen(currentT, nil); err != nil {
						resultBox.SetText(``)
						tableBox.SetText(err.Error())
					} else {
						resultBox.SetText(strings.ReplaceAll(string(result), "\t", "    "))
						url := strings.Split(currentT.Name, "_")
						for k, v := range url {
							url[k] = strings.Title(v)
						}
						if file, err := parser.ParseFile(
							token.NewFileSet(),
							fmt.Sprintf(currentT.Name+"_%d", time.Now().Nanosecond()),
							resultBox.Text, parser.ParseComments); err != nil {
							tableBox.SetText(fmt.Sprintf("generate sql failed:%v", err))
						} else {
							var sqlStr []string
							types := program.FindMatchStruct([]*ast.File{file}, func(structName string) bool {
								match, err := filepath.Match(strings.Join(url, ""), structName)
								if err != nil {
									log.Println(err.Error())
								}
								return match
							})
							for _, typ := range types {
								ms, err := sqlorm.NewSQLGenerator(typ)
								if err != nil {
									tableBox.SetText(fmt.Sprintf("create model struct failed:%s", err.Error()))
									continue
								} else {
									if sql, err := ms.GetCreateTableSQL(currentT); err != nil {
										tableBox.SetText(fmt.Sprintf("generate sql failed:%s", err.Error()))
									} else {
										sqlStr = append(sqlStr, sql)
									}
								}
							}
							tableBox.SetText(strings.Join(sqlStr, "\n\n"))
						}
					}
					tables.CurrentTab().Content = tableBox
					tables.Refresh()
				}
			}
		}()
	} else {
		return fyne.NewContainerWithLayout(
			layout.NewGridLayout(1),
			widget.NewLabel(err.Error()),
		)
	}
	tableBox := widget.NewGroupWithScroller(configs.Text("table"))
	if len(tables.Items) > 0 {
		go func() {
			currentTable <- &widget.TabItem{}
			for {
				currentTable <- tables.CurrentTab()
			}
		}()
		tables.SetTabLocation(widget.TabLocationLeading)
		tableBox.Append(tables)
	}
	return fyne.NewContainerWithLayout(
		layout.NewGridLayout(1),
		tableBox,
		widget.NewScrollContainer(resultBox),
	)
}

//QuickScreen the sql2Struct quickScreen
func QuickScreen() *fyne.Container {
	result := widget.NewMultiLineEntry()
	data := widget.NewMultiLineEntry()
	data.OnChanged = func(s string) {
		if s == "" {
			result.SetText(s)
			return
		}
		s = strings.ReplaceAll(s, "`", "")
		blocks, err := quickly.MatchStmt(strings.NewReader(s))
		if err != nil {
			result.SetText(err.Error())
			return
		}
		r := ""
		for _, v := range blocks {
			ss, err := quickly.HandleStmtBlock(v).GenType()
			if err != nil {
				r += err.Error()
				continue
			}
			r += string(ss)
		}
		result.SetText(strings.ReplaceAll(r, "\t", "    "))
	}
	data.PlaceHolder = `CREATE TABLE ` + "`" + `your_struct` + "`" + ` (
  ` + "`" + `id` + "`" + ` int(11) NOT NULL AUTO_INCREMENT,
);`
	result.SetPlaceHolder(`type YourStruct struct {
    ID uint ` + "`" + `gorm:"column:id;not null;AUTO_INCREMENT;type:int(11)" json:"id,omitempty"` + "`" + `
}`)
	return fyne.NewContainerWithLayout(
		layout.NewGridLayoutWithColumns(1),
		widget.NewScrollContainer(data),
		widget.NewScrollContainer(result),
	)
}
