package main

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/duhaifeng/simpleapi"
	"github.com/gorilla/mux"
)

func main() {
	s := new(simpleapi.ApiServer)
	s.HandRequest("GET", "/get/{getid:[0-9]+}", GetHandler)
	s.HandRequest("POST", "/pos{*}", ParamHandler)
	s.HandRequest("GET", "/user/{userid}", UrlVarHandler)
	s.HandRequest("GET", "/upload_file", OpenFilePageHandler) //用于打开上传文件的页面
	s.HandRequest("POST", "/upload_file", UpFileHandler)
	s.RegisterHandler("GET", "/struct_handler", MyStructHandler{})
	//注册拦截器
	s.RegisterInterceptor(new(MyExceptionInterceptor))

	s.Init()
	s.PrintRouteTable(true)
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
	s.StartListen("0.0.0.0", "6767")
	time.Sleep(time.Second)
}

func GetHandler(r *simpleapi.Request, w *simpleapi.Response) {
	fmt.Println(r.GetUrl())
	fmt.Println("url-var:", r.GetUrlVar("getid"))
	body, _ := r.GetBody()
	fmt.Println("body", string(body))
	w.JsonResponse(r.GetUrlVar("getid"))
}

func ParamHandler(r *simpleapi.Request, w *simpleapi.Response) {
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
	w.JsonResponse(data)
}

func UrlVarHandler(r *simpleapi.Request, w *simpleapi.Response) {
	w.SetHeader("Content-Type", "Application/json")
	fmt.Println(">>>", mux.Vars(r.GetOriReq())["userid"])
	w.JsonResponse(r.GetUrlVar("userid"))
}

const uploadHTML = `
<html>  
  <head>  
    <title>选择文件</title>
  </head>  
  <body>  
    <form enctype="multipart/form-data" action="/upload_file" method="post">  
      <input type="file" name="uploadfile" />  
      <input type="submit" value="上传文件" />  
    </form>  
  </body>  
</html>`

func OpenFilePageHandler(r *simpleapi.Request, w *simpleapi.Response) {
	w.Write([]byte(uploadHTML))
}

func UpFileHandler(r *simpleapi.Request, w *simpleapi.Response) {
	upFile, h, err := r.GetFormFile("uploadfile")
	if err != nil {
		w.JsonResponse(err.Error())
		return
	}
	defer upFile.Close()
	if h.Size != 0 {
		localFile, err := os.OpenFile("/mnt/d/"+h.Filename, os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			w.JsonResponse(err.Error())
			return
		}
		defer localFile.Close()
		_, err = io.Copy(localFile, upFile)
		if err != nil {
			w.JsonResponse(err.Error())
			return
		}
	}
	fmt.Println(">>>", r.GetFormParam("userid"))
	fmt.Println(">>>", h.Filename)
	fmt.Println(">>>", h.Size)
	w.SetHeader("Content-Type", "Application/json")
	w.JsonResponse(h.Filename)
}

type MyExceptionInterceptor struct {
	simpleapi.Interceptor
}

func (this *MyExceptionInterceptor) HandleRequest(r *simpleapi.Request) (interface{}, error) {
	fmt.Println("1111111111111111MyExceptionInterceptor", this.GetContext().GetRequestId())
	this.GetContext().SetAttachment("kkk", "vvv")
	data, err := this.CallNextProcess(r)
	if err != nil {
		fmt.Println("errrrrrrrrrrrrrrrr: ", err)
	}
	fmt.Println("22222222222222222MyExceptionInterceptor", this.GetContext().GetRequestId())
	return data, err
}

var cnt = 0
var lock = sync.Mutex{}

type MyStructHandler struct {
	simpleapi.BaseHandler
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

func (this *MyStructHandler) HandleRequest(r *simpleapi.Request) (interface{}, error) {
	time.Sleep(time.Microsecond * 1000)
	fmt.Printf(">>>>> %s %d %d\n", this.GetContext().GetRequestId(), this.flag, reflect.ValueOf(this).Pointer())
	body, _ := r.GetBody()
	fmt.Println("body", string(body))
	body2, _ := r.GetBody()
	fmt.Println("body2", string(body2))
	return this.flag, nil
}

type HandlerService struct {
	DB *ServiceDb
	simpleapi.BaseService
}

func (this *HandlerService) DoService() {
	fmt.Printf(">>>>> %s %p do service\n", this.GetContext().GetRequestId(), this)
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
	simpleapi.BaseDbOperator
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
	fmt.Printf(">>>>> %s operate db \n", this.GetContext().GetRequestId())
	newRack := new(RackInfo)
	newRack.RackName = fmt.Sprintf("dutest_%s", time.Now().Format("15:04:05"))
	newRack.PodName = "test"
	newRack.RackNetwork = "10.226.2.0/27"
	newRack.RackSubnetMask = "255.255.255.224"
	this.OrmConn().Create(newRack)
	rackInfoList := make([]*RackInfo, 0)
	err := this.OrmConn().Find(&rackInfoList).Error
	if err != nil {
		fmt.Printf(">>>>> %s fetch rack list error: %s\n", this.GetContext().GetRequestId(), err.Error())
		return
	}
	for _, rack := range rackInfoList {
		fmt.Printf(">>>>> %s rack info: %d %s\n", this.GetContext().GetRequestId(), rack.Id, rack.RackName)
	}
}
