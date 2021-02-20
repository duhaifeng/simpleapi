package db

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
)

/**
 * 基于原始sql库的数据库连接封装
 */
type MySqlProxy struct {
	conn *sql.DB
	tx   *sql.Tx
}

/**
 * 打开数据库连接
 */
func (this *MySqlProxy) Open(host, port, user, pass, database string) error {
	connStr := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8", user, pass, host, port, database)
	mysqlConn, err := sql.Open("mysql", connStr)
	if err != nil {
		return err
	}
	err = mysqlConn.Ping()
	if err != nil {
		return err
	}
	this.conn = mysqlConn
	return nil
}

/**
 * 开启数据库事务，事务会被封装为一个新的连接对象返回
 */
func (this *MySqlProxy) Begin() (*MySqlProxy, error) {
	tx, err := this.conn.Begin()
	if err != nil {
		return nil, err
	}
	txProxy := new(MySqlProxy)
	txProxy.tx = tx
	return txProxy, nil
}

/**
 * 提交数据库事务
 */
func (this *MySqlProxy) Commit() error {
	if this.tx == nil {
		return fmt.Errorf("sql: transaction is not opened")
	}
	err := this.tx.Commit()
	this.tx = nil
	return err
}

/**
 * 回滚数据库事务
 */
func (this *MySqlProxy) Rollback() error {
	if this.tx == nil {
		return fmt.Errorf("sql: transaction is not opened")
	}
	err := this.tx.Rollback()
	this.tx = nil
	return err
}

/**
 * 执行数据库查询语句
 */
func (this *MySqlProxy) Query(query string, args ...interface{}) (*sql.Rows, error) {
	if this.tx != nil {
		return this.tx.Query(query, args...)
	}
	return this.conn.Query(query, args...)
}

/**
 * 执行数据库非查询语句
 */
func (this *MySqlProxy) Exec(query string, args ...interface{}) (sql.Result, error) {
	if this.tx != nil {
		return this.tx.Exec(query, args...)
	}
	return this.conn.Exec(query, args...)
}

/**
 * 判断是否是未查询到数据错误（单独判断这个错误是因为查询不到数据不应该归为Error）
 */
func (this *MySqlProxy) IsNoRowError(err error) bool {
	return "sql: no rows in result set" == err.Error()
}

/**
 * 关闭数据库连接
 */
func (this *MySqlProxy) Close() error {
	return this.conn.Close()
}
