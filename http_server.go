package simpleapi

import (
	"fmt"
	log "github.com/duhaifeng/loglet"
	"github.com/duhaifeng/simpleapi/db"
	"github.com/gorilla/mux"
	"net/http"
	"os"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
)

var logger = log.NewLogger()

/**
 * 基于Http+Json的API短链接服务器定义
 */
type ApiServer struct {
	tokenFunnel      *TokenFunnel
	httpRouter       *mux.Router
	funcHandlerDef   []*FuncHandlerDef
	structHandlerDef []*StructHandlerDef
	interceptors     []IApiHandler
	ormConn          *db.GormProxy //管理全局数据库链接
}

/**
 * 初始化API短链接服务器
 */
func (this *ApiServer) Init() {
	//避免重复初始化
	if this.tokenFunnel != nil {
		return
	}
	this.tokenFunnel = new(TokenFunnel)
	this.tokenFunnel.Init()
	this.httpRouter = mux.NewRouter()
	this.httpRouter.NotFoundHandler = http.NotFoundHandler()
}

/**
 * 获取ApiServer内置访问令牌漏斗
 */
func (this *ApiServer) GetTokenFunnel() *TokenFunnel {
	return this.tokenFunnel
}

/**
 * 打开服务器使用的全局数据库连接
 */
func (this *ApiServer) OpenOrmConn(host, port, user, pass, database string) error {
	this.ormConn = new(db.GormProxy)
	return this.ormConn.Open(host, port, user, pass, database)
}

/**
 * 启动一个API Server的端口监听
 */
func (this *ApiServer) StartListen(addr, port string) {
	this.Init()
	this.registerFuncHandlerRoute()
	this.registerStructHandlerRoute()
	http.Handle("/", this.httpRouter)
	listen := addr + ":" + port
	logger.Info("start listen %s", listen)
	err := http.ListenAndServe(listen, nil)
	if err != nil {
		logger.Error("can not listen %s for the reason: %s", listen, err.Error())
	}
}

/**
 * 向API Server注册请求路由（一个api对应一个url，一个url对应一个handler）
 */
func (this *ApiServer) HandRequest(method, path string, handler ApiHandlerFunc) {
	this.funcHandlerDef = append(this.funcHandlerDef, &FuncHandlerDef{Method: method, Path: path, HandleFunc: handler})
}

/**
 * 注册一个拦截器，拦截器工作于Handler之前。由于拦截器实际上是面向Handler的链式调用代理，因此拦截器要能够与Handler向上转型为一致的接口类型，
 * 才能保证拦截器与Handler共同组成一个调用链
 */
func (this *ApiServer) RegisterInterceptor(interceptor IApiHandler) {
	this.interceptors = append(this.interceptors, interceptor)
}

/**
 * 向API Server注册请求路由（一个api对应一个url，一个url对应一个handler）
 */
func (this *ApiServer) RegisterHandler(method, path string, handler interface{}) {
	this.structHandlerDef = append(this.structHandlerDef, &StructHandlerDef{Method: method, Path: path, StructHandler: handler})
}

/**
 * 将API Server收到的注册路由（函数句柄）同步到底层的Http服务器中
 */
func (this *ApiServer) registerFuncHandlerRoute() {
	for i := 0; i < len(this.funcHandlerDef); i++ {
		handlerDef := this.funcHandlerDef[i]
		//促使每个url都配额生效
		this.GetTokenFunnel().AutocompleteTokenQuota(handlerDef.Path)
		handleFunc := func(w http.ResponseWriter, r *http.Request) {
			this.tokenFunnel.GetToken(r.RequestURI, nil)
			reqWrapper := new(Request)
			reqWrapper.SetOriReq(r)
			respWrapper := new(Response)
			respWrapper.Init()
			respWrapper.SetOriResp(w)
			ctx := this.constructContext(reqWrapper)
			reqWrapper.setContext(ctx)

			logger.Debug("handle api request : %s, %s", r.RequestURI, runtime.FuncForPC(reflect.ValueOf(handlerDef.HandleFunc).Pointer()).Name())
			handlerDef.HandleFunc(reqWrapper, respWrapper)
		}
		logger.Debug("register api func handler: %d <%s> %s %s", i, handlerDef.Method, handlerDef.Path, runtime.FuncForPC(reflect.ValueOf(handlerDef.HandleFunc).Pointer()).Name())
		this.httpRouter.Methods(handlerDef.Method).Path(handlerDef.Path).HandlerFunc(handleFunc)
	}
}

/**
 * 将API Server收到的注册路由（结构体句柄）同步到底层的Http服务器中
 */
func (this *ApiServer) registerStructHandlerRoute() {
	for i := 0; i < len(this.structHandlerDef); i++ {
		handlerDef := this.structHandlerDef[i]
		structHandler := handlerDef.StructHandler
		structHandlerType := reflect.TypeOf(structHandler)
		//注册路由时先尝试检查Handler类型合法性
		_, ok := reflect.New(structHandlerType).Interface().(IApiHandler)
		if !ok {
			logger.Error("url <%s>'s handler type is illegal: %s", handlerDef.Path, structHandlerType.String())
			time.Sleep(time.Second) //等待日志控制台输出
			os.Exit(1)
		}
		//促使每个url都配额生效
		this.GetTokenFunnel().AutocompleteTokenQuota(handlerDef.Path)
		handleFunc := func(w http.ResponseWriter, r *http.Request) {
			reqWrapper := new(Request)
			reqWrapper.SetOriReq(r)
			respWrapper := new(Response)
			respWrapper.Init()
			respWrapper.SetOriResp(w)
			ctx := this.constructContext(reqWrapper)
			reqWrapper.setContext(ctx)

			this.GetTokenFunnel().GetToken(r.URL.Path, ctx)
			//每次请求需要生成一个新的Handler对象，避免上下文对象被多个请求共享
			newStructHandlerVal := reflect.New(structHandlerType)
			//组装Handler内声明的所有Service Field
			newStructHandlerVal = this.assembleServiceToHandler(newStructHandlerVal, ctx)
			//TODO: 如果是POST请求，自动将BODY数据映射到对象中
			newStructHandler := newStructHandlerVal.Interface().(IApiHandler)
			newStructHandler.setContext(ctx)
			newStructHandler.setReqAndResp(reqWrapper, respWrapper)
			newStructHandler.Init()
			headerInterceptor := this.assembleInterceptors(newStructHandler, ctx)
			this.callStructHandler(headerInterceptor, ctx, reqWrapper, respWrapper)
		}
		logger.Debug("register api struct handler: %d <%s> %s %s", i, handlerDef.Method, handlerDef.Path, structHandlerType.String())
		this.httpRouter.Methods(handlerDef.Method).Path(handlerDef.Path).HandlerFunc(handleFunc)
	}
}

/**
 * 基于HTTP请求构造请求上下文对象
 */
func (this *ApiServer) constructContext(req *Request) *RequestContext {
	ctx := new(RequestContext)
	ctx.Init()
	ctx.SetUserToken(req.GetHeader(HTTP_HEADER_AUTH_TOKEN))
	ctx.SetRequestId(req.GetHeader(HTTP_HEADER_REQ_IDENTIFIER))
	ctx.SetClientFlag(req.GetHeader(HTTP_HEADER_CLIENT_FALG))
	ctx.SetClientIp(req.GetOriReq().RemoteAddr)
	return ctx
}

/**
 * 组装Handler中的Service对象
 */
func (this *ApiServer) assembleServiceToHandler(handlerVal reflect.Value, ctx *RequestContext) reflect.Value {
	handlerElem := handlerVal.Elem()
	for i := 0; i < handlerElem.NumField(); i++ {
		handlerFieldVal := handlerElem.Field(i)
		if !handlerFieldVal.IsValid() || !handlerFieldVal.CanInterface() {
			continue
		}
		handlerFieldType := handlerFieldVal.Type()
		//为了确保Service与DB相互引用时，指向的是同一个Service， 必须要求成员Field为指针类型
		if handlerFieldType.Kind() != reflect.Ptr {
			continue
		}
		//Go中父类也会被子类当成Field遍历出来，需要跳过对父类属性的重生成
		if strings.Contains(handlerFieldType.String(), "BaseHandler") {
			continue
		}
		//获取Handler中声明的公开变量类型，并通过反射的方式为其注入实例
		serviceVal := reflect.New(handlerFieldType.Elem())
		serviceObj, ok := serviceVal.Interface().(IApiService)
		if !ok {
			continue
		}
		serviceVal = this.assembleDbToService(serviceObj, serviceVal, ctx)
		//组装Service中的DB操作对象
		serviceObj.setContext(ctx)
		serviceObj.SetOrmConn(this.ormConn)
		serviceObj.Init()
		//logger.Debug("%s assemble service <%p-%s> to handler <%s>'s field", ctx.GetRequestId(), &serviceObj, handlerFieldType.Name(), handlerElem.Type().Name())
		handlerFieldVal.Set(serviceVal)
	}
	return handlerVal
}

/**
 * 组装Service中的DB操作对象
 */
func (this *ApiServer) assembleDbToService(serviceObj IApiService, serviceVal reflect.Value, ctx *RequestContext) reflect.Value {
	serviceElem := serviceVal.Elem()
	for i := 0; i < serviceElem.NumField(); i++ {
		serviceFieldVal := serviceElem.Field(i)
		if !serviceFieldVal.IsValid() || !serviceFieldVal.CanInterface() {
			continue
		}
		serviceFieldType := serviceFieldVal.Type()
		//为了确保Service与DB相互引用时，指向的是同一个Service， 必须要求成员Field为指针类型
		if serviceFieldType.Kind() != reflect.Ptr {
			continue
		}
		//Go中父类也会被子类当成Field遍历出来，需要跳过对父类属性的重生成
		if strings.Contains(serviceFieldType.String(), "BaseService") {
			continue
		}
		dbVal := reflect.New(serviceFieldType.Elem())
		dbObj, ok := dbVal.Interface().(IApiDbOperator)
		if !ok {
			continue
		}
		//logger.Debug("%s assemble db operator <%p-%s> to service <%p-%s>'s field", ctx.GetRequestId(), &dbObj, serviceFieldType.Name(), &serviceObj, serviceElem.Type().Name())
		dbObj.SetContext(ctx)
		//为了确保一个Service下所有的DB操作属于一个事务，因此需要让DB对象反引Service对象，并从中获取DB连接
		dbObj.SetService(&serviceObj)
		dbObj.Init()
		serviceFieldVal.Set(dbVal)
	}
	return serviceVal
}

/**
 * 为一个Handler组装拦截器链
 */
func (this *ApiServer) assembleInterceptors(handler IApiHandler, ctx *RequestContext) IApiHandler {
	var newInterceptors []IApiHandler
	//为了防止拦截器对象复用造成数据安全问题，所以每次请求Handler对象的关联拦截器都重新生成一份拦截器实例
	for _, interceptor := range this.interceptors {
		interceptorType := reflect.TypeOf(interceptor)
		newInterceptorVal := reflect.New(interceptorType.Elem())
		newInterceptor := newInterceptorVal.Interface().(IApiHandler)
		newInterceptor.setContext(ctx)
		newInterceptor.Init()
		newInterceptors = append(newInterceptors, newInterceptor)
	}
	//在拦截器队列最后加一个结尾拦截器，结尾拦截器纯粹用于将Handler衔接在拦截队列最后
	lastInterceptor := new(Interceptor)
	lastInterceptor.setContext(ctx)
	lastInterceptor.Init()
	lastInterceptor.SetNext(handler)
	newInterceptors = append(newInterceptors, lastInterceptor)
	//通过前一个拦截器引用后一个拦截器，让拦截器形成一个调用链
	for i := len(newInterceptors) - 1; i >= 1; i-- {
		interceptor := newInterceptors[i]
		//这里比较恶心是Go里面不同类型的指针间不等价，例如*Apple不能转换为*Fruit，即便Apple继承于Fruit
		//因此这里通额外定义了一个IApiInterceptor接口来实现拦截器对象的向上转型，这里向接口转型的目的是为了能够调用SetNext()方法
		preInterceptor := newInterceptors[i-1].(IInterceptorChain)
		preInterceptor.SetNext(interceptor)
	}
	//返回首个拦截器，这样在调用拦截器HandleRequest()方法时会触发一个链式调用
	return newInterceptors[0]
}

/**
 * 触发对一个结构体请求句柄的调用
 */
func (this *ApiServer) callStructHandler(interceptorAndHandler IApiHandler, ctx *RequestContext, r *Request, w *Response) {
	defer func() {
		err := recover()
		if err != nil {
			logger.Error(ctx.reqId+" unhandled error: %v", err)
			debug.PrintStack()
			w.JsonResponse(fmt.Sprintf("unhandled error <%s> %v", ctx.GetRequestId(), err))
		}
	}()
	resp, err := interceptorAndHandler.HandleRequest(r)
	if w.IsAlreadyResponsed() {
		return
	}
	if err != nil {
		w.JsonResponse(fmt.Sprintf("error <%s> %v", ctx.GetRequestId(), err))
	} else {
		w.JsonResponse(resp)
	}
}
