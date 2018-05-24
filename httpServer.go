package main

import "github.com/astaxie/beego"

type MainController struct {
	beego.Controller
}

func (this *MainController) Get() {
	//this.Data["Website"] = "beego.me"
	//this.Data["Email"] = "astaxie@gmail.com"
	//this.TplName = "index.tpl"
	this.Ctx.WriteString("")
}
func (this *MainController) Post() {
	bd := this.Ctx.Input.CopyBody(10000)
	bdstr := string(bd[:])
	this.Ctx.WriteString("POST Message:" + bdstr)
}
