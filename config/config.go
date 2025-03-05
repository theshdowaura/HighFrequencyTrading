package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	ExchangeLogFile  = "电信金豆换话费.log"
	ExchangeLogFile2 = "电信金豆换话费2.log"
	CacheFile        = "chinaTelecom_cache.json"
	DefaultMEXZ      = "0.5,5,6;1,10,3"
)

// Config : 存放命令行和环境变量
type Config struct {
	Jdhf string
	MEXZ string
	H    *int
}

// GlobalVars : 运行期的全局对象
type GlobalVars struct {
	Yf    string                         // 当前年月: e.g. "202503"
	Dhjl  map[string]map[string][]string // 兑换日志: 年月 -> (话费标题 -> []手机号)
	Jp    map[string]map[string]string   // 商品映射: 9->map[0.5元话费->aid_0.5], ...
	Wt    float64                        // 目标 UNIX 时间戳
	Kswt  float64                        // 偏移量
	Rs    int32
	Cache map[string]interface{}

	MorningExchanges   []string
	AfternoonExchanges []string
}

// NewConfig : 环境变量优先级>命令行
func NewConfig(cliJdhf, cliMEXZ string, cliH *int) *Config {
	cfg := &Config{}
	// 先用 CLI
	cfg.Jdhf = cliJdhf
	cfg.MEXZ = cliMEXZ
	cfg.H = cliH

	// 后看 ENV
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

// InitGlobalVars : 解析日志,缓存,MEXZ
func InitGlobalVars(cfg *Config) *GlobalVars {
	g := &GlobalVars{}
	g.Yf = time.Now().Format("200601")
	g.Dhjl = make(map[string]map[string][]string)
	g.Jp = map[string]map[string]string{"9": {}, "13": {}}
	g.Kswt = 0.1

	// 1. 读取日志
	dat, err := ioutil.ReadFile(ExchangeLogFile)
	if err == nil {
		var tmp map[string]map[string][]string
		if json.Unmarshal(dat, &tmp) == nil {
			g.Dhjl = tmp
		}
	}
	if _, ok := g.Dhjl[g.Yf]; !ok {
		g.Dhjl[g.Yf] = make(map[string][]string)
	}

	// 2. 加载缓存
	dat2, err := ioutil.ReadFile(CacheFile)
	if err == nil {
		var c map[string]interface{}
		if json.Unmarshal(dat2, &c) == nil {
			g.Cache = c
		} else {
			g.Cache = make(map[string]interface{})
		}
	} else {
		g.Cache = make(map[string]interface{})
	}

	// 3. 解析 MEXZ
	parts := strings.Split(cfg.MEXZ, ";")
	if len(parts) == 2 {
		g.MorningExchanges = parseExchanges(parts[0])
		g.AfternoonExchanges = parseExchanges(parts[1])
	} else {
		log.Println("[Warn] MEXZ 格式不正确,使用默认")
		g.MorningExchanges = parseExchanges("0.5,5,6")
		g.AfternoonExchanges = parseExchanges("1,10,3")
	}

	return g
}

// parseExchanges : "0.5,5,6" -> ["0.5元话费","5元话费","6元话费"]
func parseExchanges(raw string) []string {
	arr := strings.Split(raw, ",")
	var res []string
	for _, it := range arr {
		res = append(res, it+"元话费")
	}
	return res
}

// SaveDhjl : 保存兑换日志
func (g *GlobalVars) SaveDhjl() {
	bt, _ := json.Marshal(g.Dhjl)
	_ = ioutil.WriteFile(ExchangeLogFile, bt, 0644)
}

// SaveCache : 保存 token 缓存
func (g *GlobalVars) SaveCache() {
	bt, _ := json.Marshal(g.Cache)
	_ = ioutil.WriteFile(CacheFile, bt, 0644)
}

// Debug : 可选调试
func (cfg *Config) Debug() {
	fmt.Printf("[DEBUG] jdhf=%s MEXZ=%s H=%v\n", cfg.Jdhf, cfg.MEXZ, cfg.H)
}
