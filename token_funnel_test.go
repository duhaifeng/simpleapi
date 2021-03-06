package simpleapi

import (
	"sync"
	"testing"
	"time"
)

func TestTokenBucket(t *testing.T) {
	tokenFunnel := new(TokenFunnel)
	tokenFunnel.Init()
	tokenFunnel.SetDefaultTokenQuota(10000)
	tokenFunnel.SetTokenQuota("T1", 0)
	//tokenFunnel.SetTokenQuota("T2", 10)
	wg := new(sync.WaitGroup)

	go func() {
		wg.Add(1)
		defer wg.Done()
		wg1 := new(sync.WaitGroup)
		for i := 0; i < 1000; i++ {
			wg1.Add(1)
			go func(i int) {
				defer wg1.Done()
				tokenFunnel.GetToken("T1", nil)
				logger.Debug("T1-%d, %s", i, time.Now().String())
			}(i)
		}
		wg1.Wait()
	}()
	time.Sleep(time.Second)
	wg.Wait()
}
