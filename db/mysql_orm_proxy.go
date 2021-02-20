package db

import (
	"errors"
	"fmt"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"reflect"
	"regexp"
	"strings"
	"time"
)

/**
 * 基于gorm的数据库连接封装
 */
type GormProxy struct {
	Conn *gorm.DB
	inTx bool
}

/**
 * 打开数据库连接
 */
func (this *GormProxy) Open(host, port, user, pass, database string) error {
	connStr := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8&loc=Local&parseTime=true", user, pass, host, port, database)
	mySqlConn, err := gorm.Open("mysql", connStr)
	if err != nil {
		return err
	}
	this.Conn = mySqlConn
	this.inTx = false
	return nil
}

/**
 * 开启数据库事务，事务会被封装为一个新的连接对象返回
 */
func (this *GormProxy) Begin() (*GormProxy, error) {
	if this.Conn == nil {
		return nil, errors.New("begin transaction failed. db conn is nil")
	}
	tx := this.Conn.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("gorm: begin transaction failed.error%s", tx.Error.Error())
	}
	proxyInTx := new(GormProxy)
	proxyInTx.Conn = tx
	proxyInTx.inTx = true
	return proxyInTx, nil
}

/**
 * 提交数据库事务
 */
func (this *GormProxy) Commit() error {
	if !this.inTx {
		return fmt.Errorf("grom: transaction is not opened")
	}
	err := this.Conn.Commit().Error
	if err != nil {
		return fmt.Errorf("gorm: transaction commit failed. error:%s", err.Error())
	}
	return nil
}

/**
 * 回滚数据库事务
 */
func (this *GormProxy) Rollback() error {
	if !this.inTx {
		return fmt.Errorf("grom: transaction is not opened")
	}
	err := this.Conn.Rollback().Error
	if err != nil {
		return fmt.Errorf("grom: transaction rollback failed. error:%s", err.Error())
	}
	return nil
}

/**
 * 判断是否是未查询到数据错误（单独判断这个错误是因为查询不到数据不应该归为Error）
 */
func (this *GormProxy) IsNoRowError(err error) bool {
	return "sql: no rows in result set" == err.Error()
}

/**
 * 向数据库中批量插入数据，由于gorm不支持批量插入，因此手工实现本方法
 * 1、数组中要保证都是Struct
 * 2、要保证struct的类型一致
 * 3、保证指针和实例不能混用
 * 4、根据某一个struct反射insert语句（包括单个struct占位符的数量）
 * 5、根据数组元素数量生成占位符
 * 6、数组长度如果过长，要分批插入，比如是1000条/次
 */
func (this *GormProxy) BatchInsert(data interface{}) error {
	//判断data不能为空
	if data == nil {
		panic("param data is nil")
	}
	t := reflect.TypeOf(data)
	v := reflect.ValueOf(data)
	//判断data的类型必须为slice
	if t.Kind() != reflect.Slice {
		panic(fmt.Sprintf("data[type kind:%s] is not reflect.Slice", t.Kind()))
	}
	//判断data数组中传入的类型是struct还是ptr
	var isPtr bool
	switch t.Elem().Kind() {
	case reflect.Ptr:
		isPtr = true
	case reflect.Struct:
		isPtr = false
	default:
		panic(fmt.Sprintf("data value[%s] must be struct or struct ptr", t.Elem().Kind()))
	}
	//如果数组长度为0则直接返回
	if v.Len() == 0 {
		return nil
	}
	// 确定table_name
	tableName := getTableName(t, v)
	if tableName == "" {
		return fmt.Errorf("gorm: can't get table name")
	}
	//反射结构体中的字段为一个map[field] = reflect.Kind
	fieldMap := getStructField(t, isPtr)
	//获取table中的列
	tableFieldList := make([]string, 0)
	for _, field := range fieldMap {
		tableFieldList = append(tableFieldList, fmt.Sprintf("`%s`", camelToSnack(field)))
	}
	tableField := strings.Join(tableFieldList, ",")
	//将tableName和field拼接在一起作为sql语句的前半部分
	sql := fmt.Sprintf("insert into %s (%s) values ", tableName, tableField)
	//将sql语句拼接成 insert into {table_name} (field...) values (...),(...),(...)...的形式批量插入
	var count int
	sqlValues := make([]string, 0, 0)
	argValues := make([]interface{}, 0, 0)
	valueMapSlice := getStructValueMap(v, isPtr)
	for _, fieldValueMap := range valueMapSlice {
		count++
		//组装strVar => (?,?...?,?)
		strVar := make([]byte, 0)
		strVar = append(strVar, ' ', '(')
		for _, field := range fieldMap {
			strVar = append(strVar, '?', ',')
			argValues = append(argValues, (*fieldValueMap)[field])
		}
		strVar = strVar[0 : len(strVar)-1]
		strVar = append(strVar, ')', ' ')
		sqlValues = append(sqlValues, string(strVar))
		if count%500 == 0 {
			//每次批量500个插入
			err := this.Conn.Exec(sql+strings.Join(sqlValues, ","), argValues...).Error
			if err != nil {
				return fmt.Errorf("gorm: write data to database failed.%s\n", err)
			}
			count = 0
			sqlValues = make([]string, 0, 0)
			argValues = make([]interface{}, 0, 0)
		}
	}
	//将不足500的余下的数据一次性写入数据库
	if len(argValues) == 0 {
		return nil
	}
	err := this.Conn.Exec(sql+strings.Join(sqlValues, ","), argValues...).Error
	if err != nil {
		return fmt.Errorf("gorm: write data to database failed.%s\n", err)
	}
	return nil
}

/**
 * 将驼峰转换为下划线命名的方法
 */
func camelToSnack(old string) string {
	regex, _ := regexp.Compile("[A-Z]")
	return regex.ReplaceAllStringFunc(old, func(str string) string {
		return "_" + strings.ToLower(str)
	})[1:]
}

/**
 * 获取表名
 */
func getTableName(t reflect.Type, v reflect.Value) string {
	// 如果有TableName函数，则调用TableName函数的返回值作为表名，否则表名是结构体对应的名字
	f, ok := t.Elem().MethodByName("TableName")
	var tableName string
	if ok {
		tableName = f.Func.Call([]reflect.Value{v.Index(0)})[0].String()
	} else {
		tableName = t.Elem().String()
		tableName = camelToSnack(tableName[strings.LastIndex(tableName, ".")+1:])
	}
	return tableName
}

/**
 * 获取某个结构的字段
 */
func getStructField(t reflect.Type, isPtr bool) []string {
	//转换structType直至为非ptr类型
	var structType reflect.Type
	if isPtr {
		structType = t.Elem().Elem()
	} else {
		structType = t.Elem()
	}
	fieldMap := make([]string, 0)
	for i := 0; i < structType.NumField(); i++ {
		fieldMap = append(fieldMap, structType.Field(i).Name)
	}
	return fieldMap
}

/**
 * 将结构体的field映射为一个map
 */
func getStructValueMap(v reflect.Value, isPtr bool) []*map[string]interface{} {
	fieldMapSlice := make([]*map[string]interface{}, 0)
	for i := 0; i < v.Len(); i++ {
		var structValue reflect.Value
		if isPtr {
			structValue = v.Index(i).Elem()
		} else {
			structValue = v.Index(i)
		}
		fieldMap := make(map[string]interface{}, 0)
		for j := 0; j < structValue.NumField(); j++ {
			var (
				mapKey   = structValue.Type().Field(j).Name
				mapValue interface{}
			)
			if mapKey == "CreateTime" {
				mapValue = time.Now()
			} else {
				mapValue = structValue.FieldByName(mapKey).Interface()
			}
			fieldMap[mapKey] = mapValue
		}
		fieldMapSlice = append(fieldMapSlice, &fieldMap)
	}
	return fieldMapSlice
}

/**
 * 关闭数据库连接
 */
func (this *GormProxy) Close() error {
	err := this.Conn.Close()
	if err != nil {
		return fmt.Errorf("gorm: close database failed.error:%s", err.Error())
	}
	return nil
}
