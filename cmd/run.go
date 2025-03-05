package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"HighFrequencyTrading/config"
	"HighFrequencyTrading/exchange"
	"HighFrequencyTrading/sign"
)

// MainLogic 是程序入口
func MainLogic(cfg *config.Config) {
	log.Println("===== 高频交易系统启动 =====")

	// 1. 初始化全局配置
	g := config.InitGlobalVars(cfg)
	if cfg.Jdhf == "" {
		log.Println("[Error] 未检测到账号信息，退出")
		return
	}

	accounts := strings.Split(cfg.Jdhf, "&")
	log.Printf("检测到 %d 个账号", len(accounts))

	if cfg.H != nil {
		log.Printf("[CMD] 强制设定 h=%d", *cfg.H)
	}

	client := &http.Client{Timeout: 5 * time.Second}

	// 2. 并发处理每个账号
	var wg sync.WaitGroup
	for _, account := range accounts {
		wg.Add(1)
		go func(ac string) {
			defer wg.Done()
			processAccount(ac, g, client, cfg)
		}(account)
	}
	wg.Wait()

	// 3. 等待目标时间
	g.Mu.RLock()
	wt := g.Wt
	g.Mu.RUnlock()

	waitUntilTargetTime(wt)

	// 4. 处理日志保存
	handleExchangeLog(g)

	// 5. 推送汇总（如需的话）
	//   uid需要结合自身逻辑传入
	exchange.PushSummary(g, os.Getenv("WXPUSHER_UID"))

	log.Println("===== 高频交易系统结束 =====")
}

func processAccount(accountStr string, g *config.GlobalVars, client *http.Client, cfg *config.Config) {
	// 解析账号信息
	fields := strings.Split(accountStr, "#")
	if len(fields) < 2 {
		log.Printf("[Error] 账号格式错误: %s", accountStr)
		return
	}
	phone := fields[0]
	password := fields[1]
	uid := getUID(fields, phone)

	// 获取 token
	token := getToken(phone, password, g)
	if token == "" {
		return
	}

	// 执行交易逻辑
	executeTrading(g, phone, token, uid, client, cfg)
}

func getUID(fields []string, phone string) string {
	if len(fields) >= 3 {
		return fields[len(fields)-1]
	}
	return phone
}

// getToken 封装缓存处理逻辑：先尝试从缓存中取 token，否则重新登录获取
func getToken(phone, password string, g *config.GlobalVars) string {
	// 先读缓存
	g.Mu.RLock()
	cachedToken, ok := g.Cache[phone]
	g.Mu.RUnlock()

	if ok {
		log.Printf("[Cache] phone=%s 命中缓存", phone)
		return cachedToken
	}

	// 缓存无，则重新登录
	log.Printf("[Login] phone=%s 开始重新登录", phone)
	token, err := sign.UserLoginNormal(phone, password)
	if err != nil {
		log.Printf("[Error] phone=%s 登录失败: %v", phone, err)
		return ""
	}

	// 写缓存
	g.Mu.Lock()
	g.Cache[phone] = token
	// 保存到文件
	g.SaveCache()
	g.Mu.Unlock()

	return token
}

func executeTrading(g *config.GlobalVars, phone, token, uid string, client *http.Client, cfg *config.Config) {
	log.Printf("[Trading] phone=%s", phone)

	// 获取兑换商品列表并更新到 g.Jp
	items := getTradeItems()
	updateGlobalProducts(g, items)

	// 确定交易时间点
	tradeHour := determineTradeHour(cfg)

	// 读全局 offset
	g.Mu.RLock()
	kswt := g.Kswt
	g.Mu.RUnlock()

	targetTime := float64(exchange.CalcT(tradeHour)) + kswt

	// 读出 g.Jp[...] 需要读锁
	g.Mu.RLock()
	products := g.Jp[fmt.Sprintf("%d", tradeHour)]
	g.Mu.RUnlock()

	// 先做预热
	titles, aids := collectProductInfo(products)
	phoneNum, _ := strconv.ParseInt(phone, 10, 32)
	executeWarmupStages(g, int32(phoneNum), titles, aids, client, targetTime)

	// 正式交易
	var tradeWg sync.WaitGroup
	for title, aid := range products {
		// 检查是否已交易
		if isAlreadyTraded(g, title, int32(phoneNum)) {
			log.Printf("[Skip] %d %s 已兑换", phoneNum, title)
			continue
		}
		// 判断是否超时
		if isWaitingTooLong(targetTime) {
			log.Println("[Timeout] 等待时间超过30分钟，退出")
			return
		}
		log.Printf("[Trade] phone=%d item=%s", phoneNum, title)

		tradeWg.Add(1)
		go func(t, a, u string) {
			defer tradeWg.Done()
			// 发起兑换
			exchange.Dh(g, phone, t, a, targetTime, u, client)
		}(title, aid, uid)
	}
	tradeWg.Wait()
}

func getTradeItems() []struct {
	Title string
	ID    string
} {
	return []struct {
		Title string
		ID    string
	}{
		{"0.5元话费", "aid_0.5"},
		{"5元话费", "aid_5"},
		{"6元话费", "aid_6"},
		{"1元话费", "aid_1"},
		{"10元话费", "aid_10"},
		{"3元话费", "aid_3"},
	}
}

func updateGlobalProducts(g *config.GlobalVars, items []struct{ Title, ID string }) {
	// 先读出morningEx/afternoonEx
	g.Mu.RLock()
	morningEx := g.MorningExchanges
	afternoonEx := g.AfternoonExchanges
	g.Mu.RUnlock()

	// 写 g.Jp时需加写锁
	g.Mu.Lock()
	defer g.Mu.Unlock()
	for _, item := range items {
		if exchange.InStringArray(item.Title, morningEx) {
			g.Jp["10"][item.Title] = item.ID
		} else if exchange.InStringArray(item.Title, afternoonEx) {
			g.Jp["14"][item.Title] = item.ID
		}
	}
}

func determineTradeHour(cfg *config.Config) int {
	if cfg.H != nil {
		return *cfg.H
	}
	nowH := time.Now().Hour()
	if nowH < 11 {
		return 10
	}
	return 14
}

func collectProductInfo(products map[string]string) ([]string, []string) {
	var titles, aids []string
	for title, aid := range products {
		titles = append(titles, title)
		aids = append(aids, aid)
	}
	return titles, aids
}

func executeWarmupStages(g *config.GlobalVars, phone int32, titles, aids []string, client *http.Client, targetTime float64) {
	var wg sync.WaitGroup

	baseTime := time.Unix(int64(targetTime), 0)
	emptyRequestTime := baseTime.Add(-3 * time.Second)
	realRequestTime := baseTime.Add(-1 * time.Second)

	if time.Now().Before(emptyRequestTime) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			scheduleStage(emptyRequestTime, "抢发阶段", func() {
				exchange.DoHighFreqRequests(emptyRequestTime, fmt.Sprint(phone), client, nil)
			})
		}()
	} else {
		log.Println("[Warmup] 抢发阶段时间点已过，跳过")
	}

	if time.Now().Before(realRequestTime) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			scheduleStage(realRequestTime, "预热阶段", func() {
				exchange.DoHighFreqRealRequests(realRequestTime, fmt.Sprint(phone), titles, aids, client, nil)
			})
		}()
	} else {
		log.Println("[Warmup] 预热阶段时间点已过，跳过")
	}

	wg.Wait()
}

func scheduleStage(scheduledTime time.Time, stageName string, task func()) {
	waitDuration := time.Until(scheduledTime)
	if waitDuration > 0 {
		log.Printf("[Stage %s] 等待 %v 后启动，计划时间：%v", stageName, waitDuration, scheduledTime)
		time.Sleep(waitDuration)
	}
	log.Printf("[Stage %s] 启动时间：%v", stageName, time.Now())
	task()
}

func isAlreadyTraded(g *config.GlobalVars, title string, phone int32) bool {
	// 加读写锁访问 Dhjl
	g.Mu.RLock()
	yf := g.Yf
	dhOfMonth := g.Dhjl[yf]
	g.Mu.RUnlock()

	g.Mu.Lock()
	defer g.Mu.Unlock()

	phones, ok := dhOfMonth[title]
	if !ok {
		// 若没有此title，需要先初始化
		dhOfMonth[title] = []string{}
		return false
	}

	phoneStr := fmt.Sprint(phone)
	for _, p := range phones {
		if p == phoneStr {
			return true
		}
	}
	return false
}

func isWaitingTooLong(targetTime float64) bool {
	return float64(time.Now().Unix())-targetTime > 1800
}

func waitUntilTargetTime(targetTime float64) {
	targetTimeObj := time.Unix(int64(targetTime), int64((targetTime-math.Floor(targetTime))*1e9))
	duration := time.Until(targetTimeObj)
	if duration > 0 {
		time.Sleep(duration)
	}
}

func handleExchangeLog(g *config.GlobalVars) {
	nowMonth := time.Now().Format("200601")

	g.Mu.RLock()
	oldLog, ok := g.Dhjl[nowMonth]
	g.Mu.RUnlock()

	dhjl2 := make(map[string]map[string][]string)

	if ok {
		// 为安全考虑，将其复制到临时结构再处理
		copyOldLog := make(map[string][]string)
		g.Mu.RLock()
		for fee, phones := range oldLog {
			// 做浅拷贝
			copyOldLog[fee] = append([]string(nil), phones...)
		}
		g.Mu.RUnlock()

		for fee, phones := range copyOldLog {
			for _, phone := range phones {
				if phone == "" {
					continue
				}
				if _, ok2 := dhjl2[phone]; !ok2 {
					dhjl2[phone] = make(map[string][]string)
				}
				dhjl2[phone][nowMonth] = append(dhjl2[phone][nowMonth], fee)
			}
		}
	}

	data, _ := json.MarshalIndent(dhjl2, "", "  ")
	_ = os.WriteFile(config.ExchangeLogFile2, data, 0644)
}
