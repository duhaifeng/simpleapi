package simpleapi

import (
	"fmt"
	"github.com/gorilla/mux"
	"io"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestApiServer(t *testing.T) {
	s := new(ApiServer)
	s.HandRequest("GET", "/get/{getid:[0-9]+}", GetHandler)
	s.HandRequest("POST", "/pos{*}", ParamHandler)
	s.HandRequest("GET", "/user/{userid}", UrlVarHandler)
	s.HandRequest("POST", "/upload_file", UpFileHandler)
	s.RegisterHandler("GET", "/struct_handler", MyStructHandler{})
	//注册拦截器
	s.RegisterInterceptor(new(MyExceptionInterceptor))
	s.RegisterInterceptor(new(MyDbInterceptor))

	s.Init()
	//设置限流沙漏
	s.GetTokenFunnel().SetDefaultTokenQuota(10)
	//s.GetTokenFunnel().SetTokenQuota("/struct_handler", 10)
	//打开数据库连接
	//err := s.OpenOrmConn("127.0.0.1", "3306", "root", "123456", "test_db")
	//if err != nil {
	//	fmt.Println(err)
	//	return
	//}
	//service := new(HandlerService)
	//service = s.InitServiceInstance(service).(*HandlerService)
	//service.DoService()
	s.StartListen("0.0.0.0", "6666")
	time.Sleep(time.Second)
}

func GetHandler(r *Request, w *Response) {
	fmt.Println(r.GetUrl())
	fmt.Println("url-var:", r.GetUrlVar("getid"))
	w.FormatResponse(200, "processed by h1", r.GetUrlVar("getid"))
}

func ParamHandler(r *Request, w *Response) {
	body, err := r.GetBody()
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("body：", string(body))
	w.SetHeader("Content-Type", "Application/json")
	fmt.Println("url:", r.GetUrl())
	fmt.Println("url-k1:", r.GetUrlParam("k1"))
	fmt.Println("form-k2:", r.GetFormParam("k2"))
	data := make(map[string]interface{})
	data["k1"] = r.GetUrlParam("k1")
	data["k2"] = r.GetFormParam("k2")
	data["k3"] = []int{1, 2, 3}
	w.FormatResponse(200, "processed by h2", data)
}

func UrlVarHandler(r *Request, w *Response) {
	w.SetHeader("Content-Type", "Application/json")
	fmt.Println(">>>", mux.Vars(r.oriReq)["userid"])
	w.FormatResponse(200, "processed by h3", r.GetUrlVar("userid"))
}

func UpFileHandler(r *Request, w *Response) {
	upFile, h, err := r.GetFormFile("up_file")
	if err != nil {
		w.FormatResponse(200, "processed by h4 (err1)", err.Error())
		return
	}
	defer upFile.Close()
	if h.Size != 0 {
		localFile, err := os.OpenFile("/Users/Downloads/Template/"+h.Filename, os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			w.FormatResponse(200, "processed by h4 (err2)", err.Error())
			return
		}
		defer localFile.Close()
		_, err = io.Copy(localFile, upFile)
		if err != nil {
			w.FormatResponse(200, "processed by h4 (err3)", err.Error())
			return
		}
	}
	fmt.Println(">>>", r.GetFormParam("userid"))
	fmt.Println(">>>", h.Filename)
	fmt.Println(">>>", h.Size)
	w.SetHeader("Content-Type", "Application/json")
	w.FormatResponse(200, "processed by h4", h.Filename)
}

type MyExceptionInterceptor struct {
	Interceptor
}

func (this *MyExceptionInterceptor) HandleRequest(r *Request) (interface{}, error) {
	fmt.Println("1111111111111111MyExceptionInterceptor", this.GetContext().GetRequestId())
	this.GetContext().SetAttachment("kkk", "vvv")
	data, err := this.CallNextProcess(r)
	if err != nil {
		fmt.Println("errrrrrrrrrrrrrrrr: ", err)
	}
	fmt.Println("22222222222222222MyExceptionInterceptor", this.GetContext().GetRequestId())
	return data, err
}

type MyDbInterceptor struct {
	Interceptor
}

func (this *MyDbInterceptor) HandleRequest(r *Request) (interface{}, error) {
	fmt.Println("333333333333333333MyDbInterceptor", this.GetContext().GetRequestId())
	fmt.Println(this.GetContext().GetAttachment("kkk"))
	serviceDb := new(ServiceDb)
	serviceDb = this.InitDbOperator(serviceDb).(*ServiceDb)
	serviceDb.operateDb()
	data, err := this.CallNextProcess(r)
	fmt.Println("444444444444444444MyDbInterceptor", this.GetContext().GetRequestId())
	return data, err
}

var cnt = 0
var lock = sync.Mutex{}

type MyStructHandler struct {
	BaseHandler
	flag      int
	MyService *HandlerService
}

func (this *MyStructHandler) Init() {
	this.BaseHandler.Init()
	lock.Lock()
	defer lock.Unlock()
	cnt++
	this.flag = cnt
}

func (this *MyStructHandler) HandleRequest(r *Request) (interface{}, error) {
	time.Sleep(time.Microsecond * 1000)
	fmt.Printf(">>>>> %s %d %d\n", this.GetContext().reqId, this.flag, reflect.ValueOf(this).Pointer())
	this.MyService.DoService()
	return this.flag, nil
}

type HandlerService struct {
	DB *ServiceDb
	BaseService
}

func (this *HandlerService) DoService() {
	fmt.Printf(">>>>> %s %p do service\n", this.GetContext().reqId, this)
	err := this.BeginTransaction()
	if err != nil {
		fmt.Println(err)
		return
	}
	this.DB.operateDb()
	this.RollbackTransaction()
	this.CommitTransaction()
}

type ServiceDb struct {
	BaseDbOperator
}

type RackInfo struct {
	Id             int `gorm:"primary_key:yes"`
	RackName       string
	PodName        string
	RackNetwork    string
	RackSubnetMask string
	RackTorIp      string
}

func (*RackInfo) TableName() string {
	return "cm_rack_info"
}

func (this *ServiceDb) operateDb() {
	fmt.Printf(">>>>> %s operate db (service: %p)\n", this.GetContext().reqId, this.service)
	newRack := new(RackInfo)
	newRack.RackName = fmt.Sprintf("dutest_%s", time.Now().Format("15:04:05"))
	newRack.PodName = "test"
	newRack.RackNetwork = "10.226.2.0/27"
	newRack.RackSubnetMask = "255.255.255.224"
	this.OrmConn().Create(newRack)
	rackInfoList := make([]*RackInfo, 0)
	err := this.OrmConn().Find(&rackInfoList).Error
	if err != nil {
		fmt.Printf(">>>>> %s fetch rack list error: %s\n", this.GetContext().reqId, err.Error())
		return
	}
	for _, rack := range rackInfoList {
		fmt.Printf(">>>>> %s rack info: %d %s\n", this.GetContext().reqId, rack.Id, rack.RackName)
	}
}
