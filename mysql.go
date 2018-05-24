package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

/*
函数名：randSeq
功能：随机16进制字符串
版本：1.0
日期：2018.05.01
码农：SmartMesh
参数：
    n： 长度,整型,40"；
返回：
	hex字符串,字符串,"0x62617a2875696e7433322c626f6f6c29"。
修改记录：
*/
func RandSeq(n int) string {
	//var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	var letters = []rune("0123456789ABCD")
	rand.Seed(time.Now().Unix())
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return "0x" + string(b)
}

/*
函数名：TestMysql
功能：测试mysql
版本：1.0
日期：2018.05.01
码农：SmartMesh
参数：
返回：
修改记录：
*/
func TestMysql() {
	//连接数据库
	db, err := sql.Open("mysql", "root:mjh888@tcp(localhost:3306)/smart")
	defer db.Close()
	if err != nil {
		fmt.Println("failed to open database:", err.Error())
		return
	}
	if err := db.Ping(); err != nil {
		fmt.Println("error ping database:", err.Error())
		return
	}
	//获取表中的前十行记录
	rows, err := db.Query("SELECT * FROM test limit 10")
	defer rows.Close()
	if err != nil {
		fmt.Println("fetech data failed:", err.Error())
		return
	}
	for rows.Next() {
		var data int
		var address string
		rows.Scan(&address, &data)
		fmt.Println("address:", address, "data:", data)
	}
	// 插入一条新数据
	myAddress := RandSeq(40)
	dbcmd := fmt.Sprintf("INSERT INTO test(address,data) VALUES('%s', %d)", myAddress, rand.Intn(1000000))
	result, err := db.Exec(dbcmd)
	if err != nil {
		fmt.Println("insert data failed:", err.Error())
		return
	}
	id, err := result.LastInsertId()
	if err != nil {
		fmt.Println("fetch last insert id failed:", err.Error())
		return
	}
	fmt.Println("insert new record", id)
	// 更新一条数据
	result, err = db.Exec("UPDATE `test` SET `data`=? WHERE `address`=?", rand.Intn(1000000), "0x0000000000000000000000000000000000000000")
	if err != nil {
		fmt.Println("update data failed:", err.Error())
		return
	}
	num, err := result.RowsAffected()
	if err != nil {
		fmt.Println("fetch row affected failed:", err.Error())
		return
	}
	fmt.Println("update recors number", num)
	// 删除数据
	result, err = db.Exec("DELETE FROM `test` WHERE `data`<? and address!='0x0000000000000000000000000000000000000000'", 999)
	if err != nil {
		fmt.Println("delete data failed:", err.Error())
		return
	}
	num, err = result.RowsAffected()
	if err != nil {
		fmt.Println("fetch row affected failed:", err.Error())
		return
	}
	fmt.Println("delete record number", num)
}
