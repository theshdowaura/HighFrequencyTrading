package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	ExchangeLogFile  = "电信金豆换话费.log"
	ExchangeLogFile2 = "电信金豆换话费2.log"
	CacheFile        = "chinaTelecom_cache.json"
	DefaultMEXZ      = "0.5,5;1,10"
)

// Config : 存放命令行和环境变量的配置
type Config struct {
	Jdhf string
	MEXZ string
	H    *int
}

// GlobalVars : 运行期的全局对象
type GlobalVars struct {
	Yf    string                         // 当前年月: 例如 "202503"
	Dhjl  map[string]map[string][]string // 兑换日志：年月 -> (话费标题 -> []手机号)
	Jp    map[string]map[string]string   // 商品映射
	Wt    float64                        // 目标 UNIX 时间戳
	Kswt  float64                        // 时间偏移量
	Rs    int32
	Cache map[string]string // 缓存结构：手机号 -> token

	MorningExchanges   []string
	AfternoonExchanges []string

	Mu sync.RWMutex // 统一的读写锁
}

// NewConfig : 根据命令行参数和环境变量生成配置
func NewConfig(cliJdhf, cliMEXZ string, cliH *int) *Config {
	cfg := &Config{
		Jdhf: cliJdhf,
		MEXZ: cliMEXZ,
		H:    cliH,
	}

	// 如果有同名环境变量，则覆盖
	if envJdhf := os.Getenv("jdhf"); envJdhf != "" {
		cfg.Jdhf = envJdhf
	}
	if envMEXZ := os.Getenv("MEXZ"); envMEXZ != "" {
		cfg.MEXZ = envMEXZ
	}
	if envH := os.Getenv("CTIME"); envH != "" {
		if vv, err := strconv.Atoi(envH); err == nil {
			cfg.H = &vv
		}
	}

	if cfg.MEXZ == "" {
		cfg.MEXZ = DefaultMEXZ
	}
	return cfg
}

// InitGlobalVars : 初始化全局变量
func InitGlobalVars(cfg *Config) *GlobalVars {
	g := &GlobalVars{
		Dhjl:  make(map[string]map[string][]string),
		Jp:    map[string]map[string]string{"10": {}, "14": {}},
		Cache: make(map[string]string),
	}

	g.Yf = time.Now().Format("200601")
	g.Kswt = 0.1

	// 1. 读取兑换日志
	dat, err := ioutil.ReadFile(ExchangeLogFile)
	if err == nil {
		var tmp map[string]map[string][]string
		if json.Unmarshal(dat, &tmp) == nil {
			// 写入 Dhjl 需要加写锁，但此处 g 尚未被多协程共享
			g.Dhjl = tmp
		}
	}
	// 确保有当前月份key
	if _, ok := g.Dhjl[g.Yf]; !ok {
		g.Dhjl[g.Yf] = make(map[string][]string)
	}

	// 2. 加载缓存
	dat2, err := ioutil.ReadFile(CacheFile)
	if err == nil {
		var c map[string]string
		if json.Unmarshal(dat2, &c) == nil {
			g.Cache = c
		}
	}

	// 3. 解析 MEXZ
	parts := strings.Split(cfg.MEXZ, ";")
	if len(parts) == 2 {
		g.MorningExchanges = parseExchanges(parts[0])
		g.AfternoonExchanges = parseExchanges(parts[1])
	} else {
		log.Println("[Warn] MEXZ 格式不正确, 使用默认配置")
		g.MorningExchanges = parseExchanges("0.5,5")
		g.AfternoonExchanges = parseExchanges("1,10")
	}

	return g
}

// parseExchanges : 将 "0.5,5,6" 转成 ["0.5元话费","5元话费","6元话费"]
func parseExchanges(raw string) []string {
	arr := strings.Split(raw, ",")
	var res []string
	for _, it := range arr {
		res = append(res, it+"元话费")
	}
	return res
}

// SaveDhjl : 将兑换日志保存到文件
func (g *GlobalVars) SaveDhjl() {
	// 读锁即可，因为我们只需要读取 Dhjl 并序列化
	g.Mu.RLock()
	bt, _ := json.Marshal(g.Dhjl)
	g.Mu.RUnlock()

	_ = ioutil.WriteFile(ExchangeLogFile, bt, 0644)
}

// SaveCache : 将缓存保存到文件
func (g *GlobalVars) SaveCache() {
	// 同理，读锁
	g.Mu.RLock()
	bt, _ := json.Marshal(g.Cache)
	g.Mu.RUnlock()

	_ = ioutil.WriteFile(CacheFile, bt, 0644)
}

// Debug : 调试用
func (cfg *Config) Debug() {
	fmt.Printf("[DEBUG] jdhf=%s MEXZ=%s H=%v\n", cfg.Jdhf, cfg.MEXZ, cfg.H)
}
