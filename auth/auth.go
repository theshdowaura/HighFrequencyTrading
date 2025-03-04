package auth

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"HighFrequencyTrading/config"
	enc "HighFrequencyTrading/encryption"
)

// 一些 RSA 公钥、3DES Key IV 等，可从原脚本复制
var (
	PublicKeyB64 = `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDBkLT15ThVgz6/NOl6s8GNPofdWzWbCkWnkaAm7O2LjkM1H7dMvzkiqdxU02jamGRHLX/ZNMCXHnPcW/sDhiFCBN18qFvy8g6VYb9QtroI09e176s+ZCtiv7hbin2cCTj99iUpnEloZm19lwHyo69u5UMiPMpq0/XKBO8lYhN/gwIDAQAB
-----END PUBLIC KEY-----`

	PublicKeyData = `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQC+ugG5A8cZ3FqUKDwM57GM4io6JGcStivT8UdGt67PEOihLZTw3P7371+N47PrmsCpnTRzbTgcupKtUv8ImZalYk65dU8rjC/ridwhw9ffW2LBwvkEnDkkKKRi2liWIItDftJVBiWOh17o6gfbPoNrWORcAdcbpk2L+udld5kZNwIDAQAB
-----END PUBLIC KEY-----`

	DesKey = []byte("1234567`90koiuyhgtfrdews") // 24字节
	DesIV  = make([]byte, 8)                    // 8字节全0
)

// EncodePhone 模仿原脚本中对手机号做偏移处理
func EncodePhone(phone string) string {
	var sb strings.Builder
	for _, c := range phone {
		sb.WriteRune(c + 2)
	}
	return sb.String()
}

// UserLoginNormal 执行登录
func UserLoginNormal(g *config.GlobalVars, phone, password string, client *http.Client) (string, error) {
	// 构造加密参数
	alphabet := "abcdef0123456789"
	u1 := randStr(alphabet, 8)
	u2 := randStr(alphabet, 4)
	u3 := "4" + randStr(alphabet, 3)
	// ...
	timestamp := time.Now().Format("20060102150405")

	raw := fmt.Sprintf("iPhone 14 15.4.%s%s%s%s%s%s0$$$0.", u1, u2, phone, timestamp, password[:6], u3)
	rsaEnc, err := enc.EncryptRSA(PublicKeyB64, []byte(raw))
	if err != nil {
		return "", err
	}

	reqData := fmt.Sprintf(`{"headerInfos":{"code":"userLoginNormal","timestamp":"%s",...,"userLoginName":"%s"},"content":{"attach":"test","fieldData":{"loginAuthCipherAsymmertric":"%s","phoneNum":"%s","authentication":"%s"}}}`,
		timestamp, phone, rsaEnc, EncodePhone(phone), password)

	req, err := http.NewRequest("POST", "https://appgologin.189.cn:9031/login/client/userLoginNormal", bytes.NewBufferString(reqData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	bodyBytes, _ := ioutil.ReadAll(resp.Body)
	log.Printf("[UserLoginNormal] resp=%s", string(bodyBytes))

	// 假设解析出 userId, token
	userId := "mockUserId-" + phone
	token := "mockToken-" + phone

	// 缓存
	g.Cache[phone] = map[string]interface{}{
		"userId": userId,
		"token":  token,
	}
	g.SaveCache()

	// get ticket
	return GetTicket(g, phone, userId, token, client)
}

// GetTicket 通过 XML
func GetTicket(g *config.GlobalVars, phone, userId, token string, client *http.Client) (string, error) {
	encUserId, err := enc.Encrypt3DES(DesKey, DesIV, []byte(userId))
	if err != nil {
		return "", err
	}
	xmlBody := fmt.Sprintf(`<Request><HeaderInfos>...<Token>%s</Token><UserLoginName>%s</UserLoginName></HeaderInfos><Content>...<TargetId>%s</TargetId></Content></Request>`,
		token, phone, encUserId)

	req, err := http.NewRequest("POST", "https://appgologin.189.cn:9031/map/clientXML", strings.NewReader(xmlBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "CtClient;10.4.1;Android;13...")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bd, _ := ioutil.ReadAll(resp.Body)
	// 用正则提取 <Ticket>xxxx</Ticket>
	re := regexp.MustCompile(`<Ticket>(.*?)</Ticket>`)
	matches := re.FindStringSubmatch(string(bd))
	if len(matches) < 2 {
		return "", fmt.Errorf("no ticket found")
	}
	ticketHex := matches[1]
	plain, err := enc.Decrypt3DES(DesKey, DesIV, ticketHex)
	if err != nil {
		return "", err
	}
	return plain, nil
}

// randStr 简易随机
func randStr(chars string, length int) string {
	out := make([]byte, length)
	for i := 0; i < length; i++ {
		out[i] = chars[int(time.Now().UnixNano())%len(chars)]
	}
	return string(out)
}
