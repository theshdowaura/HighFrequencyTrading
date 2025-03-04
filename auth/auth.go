package auth

import (
	"bytes"
	"errors"
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

// RSA 公钥、3DES Key、IV 等
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

// EncodePhone : 原脚本对手机号做偏移处理
func EncodePhone(phone string) string {
	var sb strings.Builder
	for _, c := range phone {
		sb.WriteRune(c + 2)
	}
	return sb.String()
}

// UserLoginNormal : 执行登录，返回 ticket
func UserLoginNormal(g *config.GlobalVars, phone, password string, client *http.Client) (string, error) {
	alphabet := "abcdef0123456789"
	// 生成 UUID 数组中的5个元素
	u1 := randStr(alphabet, 8)
	u2 := randStr(alphabet, 4)
	u3 := "4" + randStr(alphabet, 3)
	u4 := randStr(alphabet, 4)
	u5 := randStr(alphabet, 12)
	// 为避免未使用 u4 和 u5 的错误，赋值给下划线
	_ = u4
	_ = u5

	timestamp := time.Now().Format("20060102150405")
	// 按照 Python 逻辑：只使用 uuid[0] 与 uuid[1] 构造登录凭证
	loginAuthCipherAsymmertric := "iPhone 14 15.4." + u1 + u2 + phone + timestamp + password[:6] + "0$$$0."

	// 使用 RSA 加密登录凭证（对应 Python 中 b64(loginAuthCipherAsymmertric)）
	rsaEnc, err := enc.EncryptRSA(PublicKeyB64, []byte(loginAuthCipherAsymmertric))
	if err != nil {
		return "", err
	}
	// 设备 UID 按需构造，这里示例使用 u1 + u2 + u3（也可根据业务调整）
	deviceUid := u1 + u2 + u3

	reqData := fmt.Sprintf(
		`{"headerInfos":{"code":"userLoginNormal","timestamp":"%s","broadAccount":"","broadToken":"","clientType":"#9.6.1#channel50#iPhone 14 Pro Max#","shopId":"20002","source":"110003","sourcePassword":"Sid98s","token":"","userLoginName":"%s"},"content":{"attach":"test","fieldData":{"loginType":"4","accountType":"","loginAuthCipherAsymmertric":"%s","deviceUid":"%s","phoneNum":"%s","isChinatelecom":"0","systemVersion":"15.4.0","authentication":"%s"}}}`,
		timestamp, phone, rsaEnc, deviceUid, EncodePhone(phone), password,
	)

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

	// 根据实际接口解析 JSON 结果，此处示例直接构造
	userId := "mockUserId-" + phone
	token := "mockToken-" + phone

	// 缓存 token 信息
	g.Cache[phone] = map[string]interface{}{
		"userId": userId,
		"token":  token,
	}
	g.SaveCache()

	// 调用 GetTicket 获取最终 ticket
	return GetTicket(g, phone, userId, token, client)
}

// GetTicket : XML 请求拿到最终票据
func GetTicket(g *config.GlobalVars, phone, userId, token string, client *http.Client) (string, error) {
	// 使用 3DES 加密 userId
	encUserId, err := enc.Encrypt3DES(DesKey, DesIV, []byte(userId))
	if err != nil {
		return "", err
	}

	// 构造 XML 请求数据，时间格式与 Python 中 "%Y%m%d%H%M%S" 一致
	timestamp := time.Now().Format("20060102150405")
	xmlBody := "<Request>" +
		"<HeaderInfos>" +
		"<Code>getSingle</Code>" +
		"<Timestamp>" + timestamp + "</Timestamp>" +
		"<BroadAccount></BroadAccount>" +
		"<BroadToken></BroadToken>" +
		"<ClientType>#9.6.1#channel50#iPhone 14 Pro Max#</ClientType>" +
		"<ShopId>20002</ShopId>" +
		"<Source>110003</Source>" +
		"<SourcePassword>Sid98s</SourcePassword>" +
		"<Token>" + token + "</Token>" +
		"<UserLoginName>" + phone + "</UserLoginName>" +
		"</HeaderInfos>" +
		"<Content>" +
		"<Attach>test</Attach>" +
		"<FieldData>" +
		"<TargetId>" + encUserId + "</TargetId>" +
		"<Url>4a6862274835b451</Url>" +
		"</FieldData>" +
		"</Content>" +
		"</Request>"

	// 构造请求，设置 user-agent 与 Content-Type
	req, err := http.NewRequest("POST", "https://appgologin.189.cn:9031/map/clientXML", strings.NewReader(xmlBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("user-agent", "CtClient;10.4.1;Android;13;22081212C;NTQzNzgx!#!MTgwNTg1")
	req.Header.Set("Content-Type", "text/xml;charset=UTF-8")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	bodyStr := string(bodyBytes)

	// 使用正则表达式提取 <Ticket>...</Ticket>
	re := regexp.MustCompile(`<Ticket>(.*?)</Ticket>`)
	matches := re.FindStringSubmatch(bodyStr)
	if len(matches) == 0 {
		return "", errors.New("no ticket found in response")
	}
	// 解密 Ticket 字符串
	ticket, err := enc.Decrypt3DES(DesKey, DesIV, matches[1])
	if err != nil {
		return "", err
	}
	return ticket, nil
}

// randStr : 简易随机字符串
func randStr(chars string, length int) string {
	out := make([]byte, length)
	t := time.Now().UnixNano()
	for i := 0; i < length; i++ {
		out[i] = chars[int(t)%len(chars)]
		t++
	}
	return string(out)
}
