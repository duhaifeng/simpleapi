package main

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/duhaifeng/simpleapi"
	"github.com/gorilla/mux"
)

func main() {
	s := new(simpleapi.ApiServer)
	s.HandRequest("GET", "/get/{getid:[0-9]+}", OneGetFuncHandler)
	s.HandRequest("POST", "/pos{*}", ParamFuncHandler)
	s.HandRequest("GET", "/user/{userid}", UrlVarFuncHandler)
	s.HandRequest("GET", "/upload_file", OpenFilePageFuncHandler) //用于打开上传文件的页面
	s.HandRequest("POST", "/upload_file", UpFileFuncHandler)

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

func OneGetFuncHandler(r *simpleapi.Request, w *simpleapi.Response) {
	fmt.Println(r.GetUrl())
	fmt.Println("url-var:", r.GetUrlVar("getid"))
	body, _ := r.GetBody()
	fmt.Println("body", string(body))
	w.JsonResponse(r.GetUrlVar("getid"))
}

func ParamFuncHandler(r *simpleapi.Request, w *simpleapi.Response) {
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

func UrlVarFuncHandler(r *simpleapi.Request, w *simpleapi.Response) {
	w.SetHeader("Content-Type", "Application/json")
	fmt.Println(">>>", mux.Vars(r.GetOriReq())["userid"])
	w.JsonResponse(r.GetUrlVar("userid"))
}

func OpenFilePageFuncHandler(r *simpleapi.Request, w *simpleapi.Response) {
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

	w.Write([]byte(uploadHTML))
}

func UpFileFuncHandler(r *simpleapi.Request, w *simpleapi.Response) {
	upFile, handler, err := r.GetFormFile("uploadfile")
	if err != nil {
		w.JsonResponse(err.Error())
		return
	}
	defer upFile.Close()

	fmt.Println("-----------")
	fmt.Println("receive file:", handler.Filename, handler.Size)
	if handler.Size == 0 {
		w.JsonResponse(errors.New("file size is zero."))
		return
	}

	localFile, err := os.OpenFile("/mnt/d/"+handler.Filename, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		fmt.Println("upload file failed: ", err.Error())
		w.JsonResponse(fmt.Sprintln("upload file failed:", err.Error()))
		return
	}
	defer localFile.Close()

	fhash := md5.New()
	//利用io.TeeReader在读取文件内容时计算hash值
	fileSize, err := io.Copy(localFile, io.TeeReader(upFile, fhash))
	if err != nil {
		w.JsonResponse(fmt.Sprintln("write file to disk failed:", err.Error()))
		return
	}
	hstr := hex.EncodeToString(fhash.Sum(nil))
	fmt.Println("upload success: ", handler.Filename, fileSize, hstr)

	w.SetHeader("Content-Type", "Application/json")
	w.JsonResponse(fmt.Sprint("upload success: ", handler.Filename, fileSize, hstr))
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
