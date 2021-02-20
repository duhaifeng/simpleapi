package simpleapi

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
)

/**
 * 定义API Server收到请求的处理Handler格式
 */
type ApiHandlerFunc func(r *Request, w *Response)

/**
 * 存放函数请求句柄定义
 */
type FuncHandlerDef struct {
	Method     string
	Path       string
	HandleFunc ApiHandlerFunc
}

/**
 * 存放结构体请求句柄定义
 */
type StructHandlerDef struct {
	Method        string
	Path          string
	StructHandler interface{} //此处存放的必须是IHandleRequest接口，由于反射缘故，所以此处改用interface{}存放
}

/**
 * API Server Handler所使用的Request对象封装
 */
type Request struct {
	oriReq *http.Request
	BaseDefine
}

/**
 * 获取原始的Http Request对象
 */
func (req *Request) GetOriReq() *http.Request {
	return req.oriReq
}

/**
 * 重设原始的Http Request对象
 */
func (req *Request) SetOriReq(oriReq *http.Request) error {
	req.oriReq = oriReq
	//return req.oriReq.ParseForm() //注意调用这句会导致GetBody()返回空，所以GetFormParam()中自行调用了ParseForm()
	return nil
}

/**
 * 获取Http请求的头部
 */
func (req *Request) GetHeader(key string) string {
	return req.oriReq.Header.Get(key)
}

/**
 * 设置Http请求的头部
 */
func (req *Request) SetHeader(key, value string) {
	req.oriReq.Header.Set(key, value)
}

/**
 * 获取请求的原始的Http URL
 */
func (req *Request) GetUrl() *url.URL {
	return req.oriReq.URL
}

/**
 * 获取请求URL中包含的变量
 */
func (req *Request) GetUrlVar(key string) string {
	return mux.Vars(req.oriReq)[key]
}

/**
 * 获取请求URL中包含的参数
 */
func (req *Request) GetUrlParam(key string) string {
	return req.oriReq.URL.Query().Get(key)
}

/**
 * 获取请求Form中包含的参数
 */
func (req *Request) GetFormParam(key string) string {
	err := req.oriReq.ParseForm()
	if err != nil {
		return err.Error()
	}
	return req.oriReq.FormValue(key)
}

/**
 * 获取Http请求的Body（部分客户端请求的形式是通过Body传递Json定义）
 */
func (req *Request) GetBody() ([]byte, error) {
	defer req.oriReq.Body.Close()
	body, err := ioutil.ReadAll(req.oriReq.Body)
	return body, err
}

/**
 * 从请求Form转化到上传文件
 */
func (req *Request) GetFormFile(key string) (multipart.File, *multipart.FileHeader, error) {
	return req.oriReq.FormFile(key)
}

/**
 * API Server Handler所使用的Response对象封装
 */
type Response struct {
	oriResp http.ResponseWriter
}

/**
 * 重设原始的Http Request对象
 */
func (resp *Response) SetOriResp(oriResp http.ResponseWriter) {
	resp.oriResp = oriResp
}

/**
 * 设置向客户端响应的头部信息
 */
func (resp *Response) SetHeader(key, value string) {
	resp.oriResp.Header().Set(key, value)
}

/**
 * 向客户端写回响应状态码
 */
func (resp *Response) WriteHeader(h int) {
	resp.oriResp.WriteHeader(h)
}

/**
 * 向客户端写回响应码
 */
func (resp *Response) Write(body []byte) (int, error) {
	return resp.oriResp.Write(body)
}

/**
 * 将响应消息转换为统一的Json格式写回客户端：{code:200, data:{xx}, message:"xx"}
 */
func (resp *Response) FormatResponse(httpCode int, message string, data interface{}) (int, error) {
	resp.SetHeader("Content-Type", "Application/json")
	respData := make(map[string]interface{})
	respData["code"] = httpCode
	respData["message"] = message
	respData["data"] = data
	body, err := json.Marshal(respData)
	if err != nil {
		resp.oriResp.WriteHeader(500)
		resp.oriResp.Write([]byte(err.Error()))
		return -1, err
	}
	resp.WriteHeader(http.StatusOK)
	return resp.Write(body)
}

/*
 * 主机信息导出使用
 */
func (resp *Response) FormatFileResponse(httpCode int, fileName string, data interface{}) (int, error) {
	resp.SetHeader("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	resp.SetHeader("Content-Type", "application/octet-stream")
	resp.WriteHeader(httpCode)
	file := data.([]byte)
	return resp.Write(file)
}
