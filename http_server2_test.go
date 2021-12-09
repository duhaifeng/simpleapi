package simpleapi

import (
	"fmt"
	"testing"
	"time"
)

func TestCrossServiceServer(t *testing.T) {
	s := new(ApiServer)
	s.RegisterHandler("GET", "/test", OneMStructHandler{})
	s.Init()
	//设置限流沙漏
	s.GetTokenFunnel().SetDefaultTokenQuota(10)
	s.StartListen("0.0.0.0", "6767")
	time.Sleep(time.Second)
}

type OneMStructHandler struct {
	OneService *OneService
	BaseHandler
}

func (this *OneMStructHandler) HandleRequest(r *Request) (interface{}, error) {
	this.OneService.DoService()
	return nil, nil
}

type OneService struct {
	DB  *OneDb
	Two *TwoService
	BaseService
}

func (this *OneService) DoService() {
	fmt.Printf(">>>>> one service\n")
	this.DB.OperateDb()
	this.Two.DoService()
}

type TwoService struct {
	DB *OneDb
	BaseService
}

func (this *TwoService) DoService() {
	fmt.Printf(">>>>> two service\n")
	this.DB.OperateDb()
}

type OneDb struct {
	BaseDbOperator
}

func (this *OneDb) OperateDb() {
	fmt.Println(">>>> operate db")
}
