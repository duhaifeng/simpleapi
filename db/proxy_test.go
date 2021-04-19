package db

import (
	"fmt"
	"github.com/google/uuid"
	"testing"
)

type GatewayUserList []*GatewayUserEntry

type GatewayUserEntry struct {
	Id           int `gorm:"primary_key:yes"`
	UserId       string
	UserName     string
	UserPassword string
}

func (*GatewayUserEntry) TableName() string {
	return "gw_user"
}

func TestSqliteTransaction(t *testing.T) {
	proxy := new(GormProxy)
	err := proxy.Open("sqlite3", "/Users/duhaifeng/test-sqlite", "", "", "", "")
	if err != nil {
		fmt.Println(err)
		return
	}
	proxy.Begin()
	gwUser := new(GatewayUserEntry)
	rndUuid, err := uuid.NewRandom()
	if err != nil {
		fmt.Println("create uuid failed:", err)
	}
	gwUser.UserId = rndUuid.String()
	gwUser.UserName = rndUuid.String()
	gwUser.UserPassword = "xxxxxxxx"
	err = proxy.Conn.Create(gwUser).Error
	if err != nil {
		fmt.Println("[db error] add gateway user failed: ", err.Error())
		return
	}
	var userList GatewayUserList
	err = proxy.Conn.Find(&userList).Error
	if err != nil {
		fmt.Println("[db error] get user list error: ", err.Error())
		return
	}
	for _, user := range userList {
		fmt.Println(user)
	}
	proxy.Rollback()
}

func TestMySqlTransaction(t *testing.T) {
	proxy := new(MySqlProxy)
	err := proxy.Open("127.0.0.1", "3306", "root", "123456", "test_db")
	if err != nil {
		fmt.Println(err)
		return
	}
	selectMySQLTable(proxy)
	proxy.Begin()
	_, err = proxy.Exec("insert into t1(c1) values ('v1')")
	if err != nil {
		fmt.Println(err)
		return
	}
	proxy.Rollback()

	proxy.Begin()
	selectMySQLTable(proxy)
	_, err = proxy.Exec("insert into t1(c1) values ('v2')")
	if err != nil {
		fmt.Println(err)
		return
	}
	proxy.Commit()
	selectMySQLTable(proxy)
}

func selectMySQLTable(proxy *MySqlProxy) {
	fmt.Println("--------------------------------")
	rows, err := proxy.Query("select * from user")
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
