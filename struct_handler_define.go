package simpleapi

import (
	"fmt"
	"github.com/duhaifeng/simpleapi/db"
	"gorm.io/gorm"
)

/**
 * Handler、Service、DB层的基类定义，方法形态要与IApiHandler接口一致
 */
type BaseDefine struct {
	ctx *RequestContext
}

/**
 * 实现IApiHandler接口定义的Init方法
 */
func (this *BaseDefine) Init() {}

/**
 * 设置上下文对象（实现IApiHandler接口定义方法）
 */
func (this *BaseDefine) setContext(ctx *RequestContext) {
	this.ctx = ctx
}

/**
 * 获取上下文对象（实现IApiHandler接口定义方法）
 */
func (this *BaseDefine) GetContext() *RequestContext {
	//如果ctx为nil，可能是在一些单元测试的场景直接构建了Service对象，为了避免打log时GetContext().GetRequestId()出错，这里构造一个临时ctx
	if this.ctx == nil {
		tmpCtx := new(RequestContext)
		tmpCtx.Init()
		tmpCtx.reqId = "<no-request-id>"
		return tmpCtx
	}
	return this.ctx
}

/**
 * 定义Handler的方法结构
 */
type IApiHandler interface {
	Init()
	setContext(ctx *RequestContext)
	setReqAndResp(r *Request, w *Response)
	GetContext() *RequestContext
	HandleRequest(r *Request) (interface{}, error)
}

/**
 * Handler层基类定义
 */
type BaseHandler struct {
	req  *Request
	resp *Response
	BaseDefine
}

/**
 * 设置当前Handler的请求和响应对象，用于在HandleRequest以外的方法能获取到这两个对象
 */
func (this *BaseHandler) setReqAndResp(r *Request, w *Response) {
	this.req = r
	this.resp = w
}

/**
 * 获取当前Handler的请求对象
 */
func (this *BaseHandler) GetRequest() *Request {
	return this.req
}

/**
 * 获取当前Handler的响应对象
 */
func (this *BaseHandler) GetResponse() *Response {
	return this.resp
}

/**
 * 定义拦截器链的方法结构
 */
type IInterceptorChain interface {
	SetNext(next IApiHandler)
}

/**
 * 拦截器层基类定义，注意：为了使拦截器与Handler能够够成共同的链式结构，所以要求Interceptor与Handler要接口一致，
 * 因此Interceptor继承自BaseHandler，而非BaseDefine
 */
type Interceptor struct {
	next IApiHandler
	BaseHandler
}

/**
 * 设置当前拦截器的后一个拦截器
 */
func (this *Interceptor) SetNext(next IApiHandler) {
	this.next = next
}

/**
 * 调用当前拦截器的后一个拦截器
 */
func (this *Interceptor) CallNextProcess(r *Request) (interface{}, error) {
	if this.next == nil {
		return nil, fmt.Errorf("inteceptor is nil")
	}
	return this.next.HandleRequest(r)
}

/**
 * 定义拦截器主业务逻辑方法，拦截器默认的业务逻辑就是调用下一个拦截器
 */
func (this *Interceptor) HandleRequest(r *Request) (interface{}, error) {
	return this.CallNextProcess(r)
}

/**
 * 定义Service的方法结构
 */
type IApiService interface {
	Init()
	setContext(*RequestContext)
	SetOrmConn(*db.GormProxy)
	GetOrmConn() *db.GormProxy
	GetContext() *RequestContext
	OpenOrmConnForTest(host, port, user, pass, database string) error
}

/**
 * Service层基类定义
 */
type BaseService struct {
	//Service层需要实现事务管理，因此DB连接托管在本层
	ormConn   *db.GormProxy
	ormTxConn *db.GormProxy
	BaseDefine
}

/**
 * 打开一个临时的数据库，主要用在单元测试的场景，方便对Service的方法直接测试
 */
func (this *BaseService) OpenOrmConnForTest(host, port, user, pass, database string) error {
	this.ormConn = new(db.GormProxy)
	return this.ormConn.OpenMySQL(host, port, user, pass, database)
}

/**
 * 设置Gorm数据库连接对象
 */
func (this *BaseService) SetOrmConn(ormConn *db.GormProxy) {
	//logger.Debug("%s set orm conn %p %v", this.GetContext().GetRequestId(), this, ormConn)
	this.ormConn = ormConn
}

/**
 * 获取Gorm数据库连接对象
 */
func (this *BaseService) GetOrmConn() *db.GormProxy {
	//logger.Debug("%s get orm transaction %p %v", this.GetContext().GetRequestId(), this, this.ormTxConn)
	if this.ormTxConn != nil {
		return this.ormTxConn
	}
	return this.ormConn
}

/**
 * 打开数据库事务
 */
func (this *BaseService) BeginTransaction() error {
	//if this.ormTxConn != nil {
	//	return errors.New("db transaction have already been opened")
	//}
	var err error
	this.ormTxConn, err = this.ormConn.Begin()
	if err != nil {
		return err
	}
	logger.Debug("%s orm transaction opened %p %v", this.GetContext().GetRequestId(), this, this.ormTxConn)
	return nil
}

/**
 * 提交数据库事务
 */
func (this *BaseService) CommitTransaction() error {
	logger.Debug("%s commit orm transaction %p %v", this.GetContext().GetRequestId(), this, this.ormTxConn)
	return this.ormTxConn.Commit()
}

/**
 * 回滚数据库事务
 */
func (this *BaseService) RollbackTransaction() error {
	logger.Debug("%s rollback orm transaction %p %v", this.GetContext().GetRequestId(), this, this.ormTxConn)
	return this.ormTxConn.Rollback()
}

/**
 * 定义DB操作层的方法结构
 */
type IApiDbOperator interface {
	Init()
	setContext(*RequestContext)
	SetService(*IApiService)
	GetContext() *RequestContext
	OpenOrmConnForTest(host, port, user, pass, database string) error
}

/**
 * DB操作层基类定义
 */
type BaseDbOperator struct {
	ormConnForTest *db.GormProxy
	service        *IApiService
	BaseDefine
}

/**
 * 将Service对象反向引用给DB对象，用于DB对象获取数据库连接等信息
 */
func (this *BaseDbOperator) SetService(service *IApiService) {
	this.service = service
}

/**
 * 打开一个临时的数据库，主要用在单元测试的场景，方便对Service的方法直接测试
 */
func (this *BaseDbOperator) OpenOrmConnForTest(host, port, user, pass, database string) error {
	this.ormConnForTest = new(db.GormProxy)
	return this.ormConnForTest.OpenMySQL(host, port, user, pass, database)
}

/**
 * 获取操作数据库的ORM连接
 */
func (this *BaseDbOperator) OrmConn() *gorm.DB {
	//如果打开了测试用的DB连接，则优先使用
	if this.ormConnForTest != nil {
		return this.ormConnForTest.Conn
	} else {
		return (*this.service).GetOrmConn().Conn
	}
}
