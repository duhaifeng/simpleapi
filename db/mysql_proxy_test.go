package db

import (
	"fmt"
	"testing"
)

func TestMySqlTransaction(t *testing.T) {
	proxy := new(MySqlProxy)
	err := proxy.Open("127.0.0.1", "3306", "root", "123456", "test_db")
	if err != nil {
		fmt.Println(err)
		return
	}
	selectTable(proxy)
	proxy.Begin()
	_, err = proxy.Exec("insert into t1(c1) values ('v1')")
	if err != nil {
		fmt.Println(err)
		return
	}
	proxy.Rollback()

	proxy.Begin()
	selectTable(proxy)
	_, err = proxy.Exec("insert into t1(c1) values ('v2')")
	if err != nil {
		fmt.Println(err)
		return
	}
	proxy.Commit()
	selectTable(proxy)
}

func selectTable(proxy *MySqlProxy) {
	fmt.Println("--------------------------------")
	rows, err := proxy.Query("select * from t1")
	if err != nil {
		fmt.Println(err)
		return
	}
	for rows.Next() {
		c1 := ""
		rows.Scan(&c1)
		fmt.Println(c1)
	}
}
