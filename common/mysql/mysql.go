package mysql

import (
	"database/sql"
	"errors"
	"strings"

	_ "github.com/go-sql-driver/mysql"

	"github.com/henrylee2cn/pholcus/common/util"
	"github.com/henrylee2cn/pholcus/config"
	"github.com/henrylee2cn/pholcus/logs"
)

/************************ Mysql 输出 ***************************/
//sql转换结构体
type MyTable struct {
	tableName        string
	columnNames      [][2]string // 标题字段
	rows             [][]string  // 多行数据
	sqlCode          string
	customPrimaryKey bool
	*sql.DB
}

var (
	db       *sql.DB
	err      error
	stmtChan = make(chan bool, config.MYSQL_CONN_CAP)
)

func DB() (*sql.DB, error) {
	// if db == nil || err != nil {
	// 	db, err = sql.Open("mysql", config.MYSQL.CONN_STR+"/"+config.MYSQL.DB+"?charset=utf8")
	// 	if err != nil {
	// 		logs.Log.Error("Mysql：%v\n", err)
	// 		return db, err
	// 	}
	// 	db.SetMaxOpenConns(config.MYSQL.MAX_CONNS)
	// 	db.SetMaxIdleConns(config.MYSQL.MAX_CONNS / 2)
	// }
	return db, err
}

func Refresh() {
	db, err = sql.Open("mysql", config.MYSQL_CONN_STR+"/"+config.DB_NAME+"?charset=utf8")
	if err != nil {
		logs.Log.Error("Mysql：%v\n", err)
		return
	}
	db.SetMaxOpenConns(config.MYSQL_CONN_CAP)
	db.SetMaxIdleConns(config.MYSQL_CONN_CAP)
	if err = db.Ping(); err != nil {
		logs.Log.Error("Mysql：%v\n", err)
	}
}

func New(db *sql.DB) *MyTable {
	return &MyTable{
		DB: db,
	}
}

//设置表名
func (self *MyTable) SetTableName(name string) *MyTable {
	self.tableName = name
	return self
}

//设置表单列
func (self *MyTable) AddColumn(names ...string) *MyTable {
	for _, name := range names {
		name = strings.Trim(name, " ")
		idx := strings.Index(name, " ")
		self.columnNames = append(self.columnNames, [2]string{string(name[:idx]), string(name[idx+1:])})
	}
	return self
}

//设置主键的语句（可选）
func (self *MyTable) CustomPrimaryKey(primaryKeyCode string) *MyTable {
	self.AddColumn(primaryKeyCode)
	self.customPrimaryKey = true
	return self
}

//生成"创建表单"的语句，执行前须保证SetTableName()、AddColumn()已经执行
func (self *MyTable) Create() *MyTable {
	if len(self.columnNames) == 0 {
		return self
	}
	self.sqlCode = `create table if not exists ` + self.tableName + `(`
	if !self.customPrimaryKey {
		self.sqlCode += `id int(12) not null primary key auto_increment,`
	}
	for _, title := range self.columnNames {
		self.sqlCode += title[0] + ` ` + title[1] + `,`
	}
	self.sqlCode = string(self.sqlCode[:len(self.sqlCode)-1])
	self.sqlCode += `);`

	stmtChan <- true
	defer func() {
		<-stmtChan
	}()
	stmt, err := self.DB.Prepare(self.sqlCode)
	util.CheckErr(err)

	_, err = stmt.Exec()
	util.CheckErr(err)
	return self
}

//设置插入的1行数据
func (self *MyTable) AddRow(value []string) *MyTable {
	self.rows = append(self.rows, value)
	return self
}

//向sqlCode添加"插入1行数据"的语句，执行前须保证Create()、AddRow()已经执行
//insert into table1(field1,field2) values(rows[0]),(rows[1])...
func (self *MyTable) Update() error {
	if len(self.rows) == 0 {
		return errors.New("Mysql更新内容为空")
	}

	self.sqlCode = `insert into ` + self.tableName + `(`
	if len(self.columnNames) != 0 {
		for _, v := range self.columnNames {
			self.sqlCode += "`" + v[0] + "`,"
		}
		self.sqlCode = self.sqlCode[:len(self.sqlCode)-1] + `)values`
	}
	for _, row := range self.rows {
		self.sqlCode += `(`
		for _, v := range row {
			v = strings.Replace(v, `"`, `\"`, -1)
			self.sqlCode += `"` + v + `",`
		}
		self.sqlCode = self.sqlCode[:len(self.sqlCode)-1] + `),`
	}
	self.sqlCode = self.sqlCode[:len(self.sqlCode)-1] + `;`

	stmtChan <- true
	defer func() {
		<-stmtChan
	}()

	stmt, err := self.DB.Prepare(self.sqlCode)
	if err != nil {
		return err
	}

	_, err = stmt.Exec()
	if err != nil {
		return err
	}

	// 清空临时数据
	self.rows = [][]string{}

	return nil
}

// 获取全部数据
func (self *MyTable) SelectAll() (*sql.Rows, error) {
	if self.tableName == "" {
		return nil, errors.New("表名不能为空")
	}
	self.sqlCode = `select * from ` + self.tableName + `;`
	return self.DB.Query(self.sqlCode)
}
