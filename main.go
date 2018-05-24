package main

import (
	"fmt"

	"github.com/astaxie/beego"
)

func main() {

	TestMysql()
	EthApiTest()
	fmt.Println("StrSha3:", StrSha3("baz(uint32,bool)"))
	Sigtest("0x1a9ec3b0b807464e6d3398a59d6b0a369bf422fa")
	beego.Router("/", &MainController{})
	beego.Run()
}
