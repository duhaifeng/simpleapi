package simpleapi

import (
	"fmt"
	"github.com/duhaifeng/simpleapi/db"
	"github.com/jinzhu/gorm"
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
func (this *BaseDefine) Init() {

}

/**
 * 设置上下文对象（实现IApiHandler接口定义方法）
 */
func (this *BaseDefine) SetContext(ctx *RequestContext) {
	this.ctx = ctx
}

/**
 * 获取上下文对象（实现IApiHandler接口定义方法）
 */
func (this *BaseDefine) GetContext() *RequestContext {
	return this.ctx
}

/**
 * 定义Handler的方法结构
 */
type IApiHandler interface {
	Init()
	setApiServer(apiServer *ApiServer)
	SetContext(ctx *RequestContext)
	GetContext() *RequestContext
	HandleRequest(r *Request) (interface{}, error)
}

/**
 * 定义拦截器链的方法结构
 */
type IInterceptorChain interface {
	SetNext(next IApiHandler)
}

/**
 * 拦截器层基类定义，注意：为了使拦截器与Handler能够够成共同的链式结构，所以要求Interceptor与Handler要接口一致
 * （说白了Interceptor是Handler的外层代理）
 */
type Interceptor struct {
	apiServer *ApiServer
	next      IApiHandler
	BaseDefine
}

/**
 * 引用ApiServer对象，用于获取ApiServer提供的DbOperator初始化能力
 */
func (this *Interceptor) setApiServer(apiServer *ApiServer) {
	this.apiServer = apiServer
}

func (this *Interceptor) InitDbOperator(dbOperatorObj IApiDbOperator) interface{} {
	return this.apiServer.InitDbOperator(dbOperatorObj)
}

/**
 * 定义拦截器主业务逻辑方法，拦截器默认的业务逻辑就是调用下一个拦截器
 */
func (this *Interceptor) HandleRequest(r *Request) (interface{}, error) {
	return this.CallNextProcess(r)
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
 * Handler层基类定义
 */
type BaseHandler struct {
	apiServer *ApiServer
	BaseDefine
}

/**
 * 引用ApiServer对象，用于获取ApiServer的部分初始化能力（对于Handler而言，目前尚无必要使用）
 */
func (this *BaseHandler) setApiServer(apiServer *ApiServer) {
	this.apiServer = apiServer
}

/**
 * 定义Service的方法结构
 */
type IApiService interface {
	Init()
	SetOrmConn(*db.GormProxy)
	GetOrmConn() *db.GormProxy
	SetContext(*RequestContext)
	GetContext() *RequestContext
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
	SetService(*IApiService)
	SetContext(*RequestContext)
	GetContext() *RequestContext
}

/**
 * DB操作层基类定义
 */
type BaseDbOperator struct {
	service *IApiService
	BaseDefine
}

/**
 * 将Service对象反向引用给DB对象，用于DB对象获取数据库连接等信息
 */
func (this *BaseDbOperator) SetService(service *IApiService) {
	this.service = service
}

/**
 * 获取操作数据库的ORM连接
 */
func (this *BaseDbOperator) OrmConn() *gorm.DB {
	return (*this.service).GetOrmConn().Conn
}
