package apifw

import (
	"sync"
	"time"
)

/**
 * 访问令牌漏斗定义
 */
type TokenFunnel struct {
	defaultQuota           int
	tokenChanBucket        map[string]chan byte
	tokenQuotaBucket       map[string]int
	undefinedTokenLogCount map[string]int
	lock                   sync.Mutex
}

/**
 * 初始化访问令牌漏斗
 */
func (this *TokenFunnel) Init() {
	this.tokenChanBucket = make(map[string]chan byte)
	this.tokenQuotaBucket = make(map[string]int)
	this.undefinedTokenLogCount = make(map[string]int)
	//启动每秒投放令牌的后台worker协程
	go func() {
		for {
			time.Sleep(time.Second)
			this.fullTokenPerSec()
		}
	}()
}

/**
 * 向每个令牌管道投放令牌的Worker
 */
func (this *TokenFunnel) fullTokenPerSec() {
	this.lock.Lock()
	defer this.lock.Unlock()
	for tokenName, tokenChan := range this.tokenChanBucket {
		tokenQuota := this.GetTokenQuota(tokenName)
		//如果配额没有定义，则不发放配额
		if tokenQuota == 0 {
			logger.Warn("there is no token quota for <%s>", tokenName)
			continue
		}
		//注意：tokenQuota - len(tokenChan)不能直接放到for循环里，由于len(tokenChan)一直在变，所以会导致for循环提前退出
		quotaSupplement := tokenQuota - len(tokenChan)
		for i := 0; i < quotaSupplement; i++ {
			tokenChan <- 0
		}
	}
}

/**
 * 设定默认配额（默认配额只对注册的token name有效）
 */
func (this *TokenFunnel) SetDefaultTokenQuota(defaultQuotaPerSec int) {
	this.lock.Lock()
	defer this.lock.Unlock()
	this.defaultQuota = defaultQuotaPerSec
}

/**
 * 如果指定名字的token没有设置配额，则将其注册为使用默认配额，以确保配额生效
 */
func (this *TokenFunnel) AutocompleteTokenQuota(tokenName string) {
	_, ok := this.tokenQuotaBucket[tokenName]
	if ok {
		return
	}
	this.SetTokenQuota(tokenName, 0)
}

/**
 * 设定指定token的配额
 */
func (this *TokenFunnel) SetTokenQuota(tokenName string, tokenQuotaPerSec int) {
	this.lock.Lock()
	defer this.lock.Unlock()
	//0代表使用默认配额
	if tokenQuotaPerSec < 0 {
		tokenQuotaPerSec = 0
	}
	if tokenQuotaPerSec == 0 {
		tokenQuotaPerSec = this.defaultQuota
	}
	this.tokenQuotaBucket[tokenName] = tokenQuotaPerSec
	this.tokenChanBucket[tokenName] = make(chan byte, 100000)
}

/**
 * 获取指定token的配额，如果配额设定为0则使用默认配额
 */
func (this *TokenFunnel) GetTokenQuota(tokenName string) int {
	tokenQuota, ok := this.tokenQuotaBucket[tokenName]
	//如果没有定义令牌配额，则使用默认配额
	if ok {
		return tokenQuota
	}
	return this.defaultQuota
}

/**
 * 获取指定名称的令牌，如果当前1秒内的令牌用完，则会阻塞到下一秒发放令牌为止。
 * 对于未注册的令牌，不进行配额限制，直接放行。
 * 对于配额设定为0的令牌，使用默认配额。
 */
func (this *TokenFunnel) GetToken(tokenName string, ctx *RequestContext) {
	tokenChan, ok := this.tokenChanBucket[tokenName]
	if !ok {
		this.logUndefinedTokenName(tokenName, ctx)
		return
	}
	if len(tokenChan) == 0 {
		if ctx != nil {
			logger.Debug("%s wait access token for <%s> ", ctx.GetRequestId(), tokenName)
		} else {
			logger.Debug("wait access token for <%s> ", tokenName)
		}
	}
	<-tokenChan
}

/**
 * 对于未注册的token，进行限次日志警告
 */
func (this *TokenFunnel) logUndefinedTokenName(tokenName string, ctx *RequestContext) {
	undefinedTokenCount, ok := this.undefinedTokenLogCount[tokenName]
	if !ok {
		undefinedTokenCount = 0
	}
	if undefinedTokenCount > 10 {
		return
	}
	undefinedTokenCount++
	this.undefinedTokenLogCount[tokenName] = undefinedTokenCount
	if ctx != nil {
		logger.Warn("%s access token does not defined for <%s> ", ctx.GetRequestId(), tokenName)
	} else {
		logger.Warn("access token does not defined for <%s> ", tokenName)
	}
}
