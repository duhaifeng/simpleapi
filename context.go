package apifw

import (
	"github.com/google/uuid"
	"sync"
)

/**
 * 定义API请求的上下文对象
 */
type RequestContext struct {
	userToken     string
	callerFlag    string
	reqId         string
	clientIp      string
	clientFlag    string
	ctxAttachment *sync.Map
}

/**
 * 上下文对象初始化
 */
func (this *RequestContext) Init() {
	this.ctxAttachment = new(sync.Map)
}

/**
 * 向上下文中设置用户Token
 */
func (this *RequestContext) SetUserToken(userToken string) {
	this.userToken = userToken
}

/**
 * 从上下文中获取用户Token
 */
func (this *RequestContext) GetUserToken() string {
	return this.userToken
}

/**
 * 向上下文中设置请求ID，如果调用上游未传入，则重新生成一个
 */
func (this *RequestContext) SetRequestId(reqId string) {
	if reqId == "" {
		reqId = uuid.New().String()
	}
	this.reqId = reqId
}

/**
 * 从上下文中获取请求ID
 */
func (this *RequestContext) GetRequestId() string {
	return this.reqId
}

/**
 * 向上下文中设置客户端的标记（用来标识请求方）
 */
func (this *RequestContext) SetClientFlag(clientFlag string) {
	this.clientFlag = clientFlag
}

/**
 * 从上下文中获取客户端的标记
 */
func (this *RequestContext) GetClientFlag() string {
	return this.clientFlag
}

/**
 * 向上下文中设置客户端的IP地址
 */
func (this *RequestContext) SetClientIp(clientIp string) {
	this.clientIp = clientIp
}

/**
 * 从上下文中获取客户端的IP地址
 */
func (this *RequestContext) GetClientIp() string {
	return this.clientIp
}

/**
 * 通过上下文存储和传递附件数据
 */
func (this *RequestContext) SetAttachment(key string, value interface{}) {
	this.ctxAttachment.Store(key, value)
}

/**
 * 从上下文中获取附件数据
 */
func (this *RequestContext) GetAttachment(key string) (interface{}, bool) {
	return this.ctxAttachment.Load(key)
}

/**
 * 从上下文中删除附件数据
 */
func (this *RequestContext) RmAttachment(key string) {
	this.ctxAttachment.Delete(key)
}
