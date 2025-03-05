// 文件：cmd/cmd.go
package cmd

import (
	"encoding/json"
	"fmt"
	"log"
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

	// 1. 初始化全局配置（包括读取日志和缓存）
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

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

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

	// 3. 等待至目标时间
	waitUntilTargetTime(g.Wt)

	// 4. 处理日志保存
	handleExchangeLog(g)

	// 5. 检查推送（可选）
	exchange.CheckPushTime()

	log.Println("===== 高频交易系统结束 =====")
}

func processAccount(accountStr string, g *config.GlobalVars, client *http.Client, cfg *config.Config) {
	// 解析账号信息（格式：phone#password[#uid]）
	fields := strings.Split(accountStr, "#")
	if len(fields) < 2 {
		log.Printf("[Error] 账号格式错误: %s", accountStr)
		return
	}
	phone := fields[0]
	password := fields[1]
	uid := getUID(fields, phone)

	// 获取 token（先从缓存中取，缓存中没有则重新登录获取）
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
	// 1. 如果缓存中已有 token，直接返回
	if cachedToken, ok := g.Cache[phone]; ok {
		log.Printf("[Cache] phone=%s 命中缓存", phone)
		// 如有需要，可在此处验证 token 是否依然有效
		return cachedToken
	}

	// 2. 缓存中没有，则重新登录
	log.Printf("[Login] phone=%s 开始重新登录", phone)
	token, err := sign.UserLoginNormal(phone, password)
	if err != nil {
		log.Printf("[Error] phone=%s 登录失败: %v", phone, err)
		return ""
	}

	// 3. 登录成功后，将新 token 存入缓存并写回文件
	g.Cache[phone] = token
	g.SaveCache()

	return token
}

func executeTrading(g *config.GlobalVars, phone, token, uid string, client *http.Client, cfg *config.Config) {
	log.Printf("[Trading] phone=%s", phone)

	// 获取商品列表，并更新全局产品信息
	items := getTradeItems()
	updateGlobalProducts(g, items)

	// 确定交易时间（9点或13点）
	tradeHour := determineTradeHour(cfg)
	targetTime := calculateTargetTime(tradeHour, g)
	products := g.Jp[fmt.Sprintf("%d", tradeHour)]

	// 收集商品信息
	titles, aids := collectProductInfo(products)

	// 执行预热阶段（3秒前抢发、1秒前实际预热）
	phoneNum, _ := strconv.ParseInt(phone, 10, 32)
	executeWarmupStages(g, int32(phoneNum), titles, aids, client, targetTime)

	// 执行实际交易
	var tradeWg sync.WaitGroup
	for title, aid := range products {
		// 检查是否已兑换
		if isAlreadyTraded(g, title, int32(phoneNum)) {
			log.Printf("[Skip] %d %s 已兑换", phoneNum, title)
			continue
		}
		// 如果距离目标时间超过30秒，则退出
		if isWaitingTooLong(targetTime) {
			log.Println("[Timeout] 等待时间超过30秒，退出")
			return
		}
		log.Printf("[Trade] phone=%d item=%s", phoneNum, title)

		tradeWg.Add(1)
		go func(t, a, u string) {
			defer tradeWg.Done()
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
	for _, item := range items {
		if exchange.InStringArray(item.Title, g.MorningExchanges) {
			g.Jp["9"][item.Title] = item.ID
		} else if exchange.InStringArray(item.Title, g.AfternoonExchanges) {
			g.Jp["13"][item.Title] = item.ID
		}
	}
}

func determineTradeHour(cfg *config.Config) int {
	if cfg.H != nil {
		return *cfg.H
	}
	nowH := time.Now().Hour()
	if nowH < 11 {
		return 9
	}
	return 13
}

func calculateTargetTime(hour int, g *config.GlobalVars) float64 {
	target := exchange.CalcT(hour)
	return float64(target) + g.Kswt
}

func collectProductInfo(products map[string]string) ([]string, []string) {
	var titles, aids []string
	for title, aid := range products {
		titles = append(titles, title)
		aids = append(aids, aid)
	}
	return titles, aids
}

// executeWarmupStages 独立调度预热阶段：3秒前空请求、1秒前实际预热
func executeWarmupStages(g *config.GlobalVars, phone int32, titles, aids []string, client *http.Client, targetTime float64) {
	var wg sync.WaitGroup

	// 基准时间
	baseTime := time.Unix(int64(targetTime), 0)
	emptyRequestTime := baseTime.Add(-3 * time.Second)
	realRequestTime := baseTime.Add(-1 * time.Second)

	// 阶段1：3秒前的空请求抢发
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

	// 阶段2：1秒前的实际预热
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

// scheduleStage 在指定时间点等待后执行任务
func scheduleStage(scheduledTime time.Time, stageName string, task func()) {
	waitDuration := time.Until(scheduledTime)
	if waitDuration > 0 {
		log.Printf("[Stage %s] 等待 %v 后启动，计划时间：%v", stageName, waitDuration, scheduledTime)
		time.Sleep(waitDuration)
	}
	log.Printf("[Stage %s] 启动时间：%v", stageName, time.Now())
	task()
}

// isAlreadyTraded 判断当前商品是否已兑换（通过查日志）
func isAlreadyTraded(g *config.GlobalVars, title string, phone int32) bool {
	phones, ok := g.Dhjl[g.Yf][title]
	if !ok {
		// 当前商品尚无兑换记录，初始化为空切片
		g.Dhjl[g.Yf][title] = []string{}
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

// isWaitingTooLong 若当前时间与目标时间差超过30秒则返回 true
func isWaitingTooLong(targetTime float64) bool {
	return float64(time.Now().Unix())-targetTime > 30
}

// waitUntilTargetTime 等待直到目标时间到达
func waitUntilTargetTime(targetTime float64) {
	for {
		if float64(time.Now().Unix()) >= targetTime {
			break
		}
		time.Sleep(time.Second)
	}
}

func handleExchangeLog(g *config.GlobalVars) {
	nowMonth := time.Now().Format("200601")
	dhjl2 := make(map[string]map[string][]string)

	if oldLog, ok := g.Dhjl[nowMonth]; ok {
		for fee, phones := range oldLog {
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
