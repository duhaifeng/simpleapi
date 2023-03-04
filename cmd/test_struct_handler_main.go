package main

import (
	"fmt"
	"time"

	"github.com/duhaifeng/simpleapi"
)

func main() {
	s := new(simpleapi.ApiServer)
	s.RegisterHandler("GET", "/test", OneMStructHandler{})
	s.Init()
	//设置限流沙漏
	s.GetTokenFunnel().SetDefaultTokenQuota(10)
	s.StartListen("0.0.0.0", "6767")
	time.Sleep(time.Second)
}

type OneMStructHandler struct {
	OneService *OneService
	simpleapi.BaseHandler
}

func (this *OneMStructHandler) HandleRequest(r *simpleapi.Request) (interface{}, error) {
	this.OneService.DoService()
	return nil, nil
}

type OneService struct {
	DB  *OneDb
	Two *TwoService
	simpleapi.BaseService
}

func (this *OneService) DoService() {
	fmt.Printf(">>>>> one service\n")
	this.DB.OperateDb()
	this.Two.DoService()
}

type TwoService struct {
	DB *OneDb
	simpleapi.BaseService
}

func (this *TwoService) DoService() {
	fmt.Printf(">>>>> two service\n")
	this.DB.OperateDb()
}

type OneDb struct {
	simpleapi.BaseDbOperator
}

func (this *OneDb) OperateDb() {
	fmt.Println(">>>> operate db")
}
