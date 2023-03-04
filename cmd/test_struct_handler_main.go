package main

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/duhaifeng/simpleapi"
)

func main() {
	s := new(simpleapi.ApiServer)
	s.RegisterHandler("GET", "/test", OneStructHandler{})
	s.RegisterHandler("GET", "/struct_handler", MyStructHandler{})
	s.RegisterHandler("GET", "/upload_file", OpenFilePageHandler{}) //用于打开上传文件的页面
	s.RegisterHandler("POST", "/upload_file", UpFileHandler{})

	s.Init()
	//设置限流沙漏
	s.GetTokenFunnel().SetDefaultTokenQuota(10)
	s.StartListen("0.0.0.0", "6767")
	time.Sleep(time.Second)
}

type OneStructHandler struct {
	OneService *OneService
	simpleapi.BaseHandler
}

func (this *OneStructHandler) HandleRequest(r *simpleapi.Request) (interface{}, error) {
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

type OpenFilePageHandler struct {
	OneService *OneService
	simpleapi.BaseHandler
}

func (this *OpenFilePageHandler) HandleRequest(r *simpleapi.Request) (interface{}, error) {
	uploadHTML := `
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
	this.GetResponse().Write([]byte(uploadHTML))
	this.GetResponse().AlreadyResponsed() //告诉框架Handler已经自行处理响应，不需要返回转换的Json格式数据
	return nil, nil
}

type UpFileHandler struct {
	OneService *OneService
	simpleapi.BaseHandler
}

func (this *UpFileHandler) HandleRequest(r *simpleapi.Request) (interface{}, error) {
	upFile, handler, err := r.GetFormFile("uploadfile")
	if err != nil {
		return nil, err
	}
	defer upFile.Close()

	fmt.Println("-----------")
	fmt.Println("receive file:", handler.Filename, handler.Size)
	if handler.Size == 0 {
		return nil, errors.New("file size is zero.")
	}

	localFile, err := os.OpenFile("/mnt/d/"+handler.Filename, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		fmt.Println("upload file failed: ", err.Error())
		return nil, err
	}
	defer localFile.Close()

	fhash := md5.New()
	//利用io.TeeReader在读取文件内容时计算hash值
	fileSize, err := io.Copy(localFile, io.TeeReader(upFile, fhash))
	if err != nil {
		fmt.Println("write file to disk failed:", err.Error())
		return nil, err
	}
	hstr := hex.EncodeToString(fhash.Sum(nil))
	fmt.Println("upload success: ", handler.Filename, fileSize, hstr)

	return fmt.Sprint("upload success: ", handler.Filename, ", ", fileSize, ", ", hstr), nil
}
