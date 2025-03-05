package sign

import (
	"bytes"
	"crypto/cipher"
	"crypto/des"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	mathrand "math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// 初始化：设置随机数种子
func init() {
	mathrand.Seed(time.Now().UnixNano())
}

// 常量定义
const (
	des3Key      = "1234567`90koiuyhgtfrdews"
	publicKeyPEM = `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDBkLT15ThVgz6/NOl6s8GNPofdWzWbCkWnkaAm7O2LjkM1H7dMvzkiqdxU02jamGRHLX/ZNMCXHnPcW/sDhiFCBN18qFvy8g6VYb9QtroI09e176s+ZCtiv7hbin2cCTj99iUpnEloZm19lwHyo69u5UMiPMpq0/XKBO8lYhN/gwIDAQAB
-----END PUBLIC KEY-----`
)

// pkcs7Pad 对 data 做 PKCS7 填充
func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padText...)
}

// pkcs7Unpad 去除 PKCS7 填充
func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, errors.New("invalid data length")
	}
	padding := int(data[len(data)-1])
	if padding == 0 || padding > blockSize {
		return nil, errors.New("invalid padding")
	}
	for i := len(data) - padding; i < len(data); i++ {
		if int(data[i]) != padding {
			return nil, errors.New("invalid padding")
		}
	}
	return data[:len(data)-padding], nil
}

// Encrypt 使用 3DES CBC 模式对明文进行加密，并返回 hex 编码后的字符串
func Encrypt(text string) (string, error) {
	key := []byte(des3Key)
	block, err := des.NewTripleDESCipher(key)
	if err != nil {
		return "", err
	}
	blockSize := block.BlockSize()
	plainData := pkcs7Pad([]byte(text), blockSize)
	cipherText := make([]byte, len(plainData))
	iv := make([]byte, blockSize) // IV 为全 0
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(cipherText, plainData)
	return hex.EncodeToString(cipherText), nil
}

// Decrypt 解密 hex 编码的密文，并返回明文
func Decrypt(hexText string) (string, error) {
	cipherData, err := hex.DecodeString(hexText)
	if err != nil {
		return "", err
	}
	key := []byte(des3Key)
	block, err := des.NewTripleDESCipher(key)
	if err != nil {
		return "", err
	}
	blockSize := block.BlockSize()
	if len(cipherData)%blockSize != 0 {
		return "", errors.New("ciphertext is not a multiple of block size")
	}
	iv := make([]byte, blockSize)
	mode := cipher.NewCBCDecrypter(block, iv)
	plainData := make([]byte, len(cipherData))
	mode.CryptBlocks(plainData, cipherData)
	unpadded, err := pkcs7Unpad(plainData, blockSize)
	if err != nil {
		return "", err
	}
	return string(unpadded), nil
}

// B64 使用 RSA 公钥对 text 进行加密，并返回 base64 编码后的字符串
func B64(text string) (string, error) {
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return "", errors.New("failed to parse PEM block")
	}
	pubInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", err
	}
	pub, ok := pubInterface.(*rsa.PublicKey)
	if !ok {
		return "", errors.New("not a valid RSA public key")
	}
	cipherText, err := rsa.EncryptPKCS1v15(rand.Reader, pub, []byte(text))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(cipherText), nil
}

// EncodePhone 对 phone 中每个字符的码值加 2
func EncodePhone(phone string) string {
	var result strings.Builder
	for _, ch := range phone {
		result.WriteRune(ch + 2)
	}
	return result.String()
}

// randomSample 从 alphabet 中不放回随机选取 n 个字符
func randomSample(alphabet string, n int) string {
	runes := []rune(alphabet)
	if n > len(runes) {
		n = len(runes)
	}
	// 创建一个索引切片并打乱顺序
	indices := mathrand.Perm(len(runes))
	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.WriteRune(runes[indices[i]])
	}
	return sb.String()
}

// UserLoginNormal 模拟使用密码登录，返回 ticket 字符串（若失败返回空字符串和错误）
func UserLoginNormal(phone, password string) (string, error) {
	alphabet := "abcdef0123456789"
	uuid0 := randomSample(alphabet, 8)
	uuid1 := randomSample(alphabet, 4)
	uuid2 := "4" + randomSample(alphabet, 3)

	timestampStr := time.Now().Format("20060102150405")
	loginAuth := fmt.Sprintf("iPhone 14 15.4.%s%s%s%s%s0$$$0.", uuid0, uuid1, phone, timestampStr, password[:6])
	encryptedLoginAuth, err := B64(loginAuth)
	if err != nil {
		return "", err
	}

	payload := map[string]interface{}{
		"headerInfos": map[string]interface{}{
			"code":           "userLoginNormal",
			"timestamp":      timestampStr,
			"broadAccount":   "",
			"broadToken":     "",
			"clientType":     "#9.6.1#channel50#iPhone 14 Pro Max#",
			"shopId":         "20002",
			"source":         "110003",
			"sourcePassword": "Sid98s",
			"token":          "",
			"userLoginName":  phone,
		},
		"content": map[string]interface{}{
			"attach": "test",
			"fieldData": map[string]interface{}{
				"loginType":                  "4",
				"accountType":                "",
				"loginAuthCipherAsymmertric": encryptedLoginAuth,
				"deviceUid":                  uuid0 + uuid1 + uuid2,
				"phoneNum":                   EncodePhone(phone),
				"isChinatelecom":             "0",
				"systemVersion":              "15.4.0",
				"authentication":             password,
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	client := &http.Client{}
	req, err := http.NewRequest("POST", "https://appgologin.189.cn:9031/login/client/userLoginNormal", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 13; Build/Example) Chrome/104.0 Mobile Safari/537.36")
	req.Header.Set("Referer", "https://wapact.189.cn:9001/JinDouMall/JinDouMall_independentDetails.html")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("登录请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var respJSON map[string]interface{}
	if err := json.Unmarshal(body, &respJSON); err != nil {
		return "", err
	}

	// 遍历 JSON 结构：responseData -> data -> loginSuccessResult
	responseData, ok := respJSON["responseData"].(map[string]interface{})
	if !ok {
		return "", errors.New("登录失败，未获取到 responseData")
	}
	data, ok := responseData["data"].(map[string]interface{})
	if !ok {
		return "", errors.New("登录失败，未获取到 data")
	}
	loginSuccessResult, ok := data["loginSuccessResult"].(map[string]interface{})
	if !ok {
		return "", errors.New("登录失败，未获取到 loginSuccessResult")
	}
	userId, ok := loginSuccessResult["userId"].(string)
	if !ok {
		return "", errors.New("登录失败，userId 缺失")
	}
	token, ok := loginSuccessResult["token"].(string)
	if !ok {
		return "", errors.New("登录失败，token 缺失")
	}

	ticket, err := GetTicket(phone, userId, token)
	if err != nil {
		return "", err
	}
	return ticket, nil
}

// GetTicket 根据登录返回的 userId 与 token，通过调用 getSingle 接口获取并解密 ticket
func GetTicket(phone, userId, token string) (string, error) {
	timestampStr := time.Now().Format("20060102150405")
	encryptedUserId, err := Encrypt(userId)
	if err != nil {
		return "", err
	}

	xmlPayload := fmt.Sprintf(
		`<Request><HeaderInfos>`+
			`<Code>getSingle</Code><Timestamp>%s</Timestamp>`+
			`<BroadAccount></BroadAccount><BroadToken></BroadToken>`+
			`<ClientType>#9.6.1#channel50#iPhone 14 Pro Max#</ClientType>`+
			`<ShopId>20002</ShopId><Source>110003</Source>`+
			`<SourcePassword>Sid98s</SourcePassword>`+
			`<Token>%s</Token>`+
			`<UserLoginName>%s</UserLoginName>`+
			`</HeaderInfos><Content><Attach>test</Attach>`+
			`<FieldData><TargetId>%s</TargetId>`+
			`<Url>4a6862274835b451</Url></FieldData></Content></Request>`,
		timestampStr, token, phone, encryptedUserId,
	)

	client := &http.Client{}
	req, err := http.NewRequest("POST", "https://appgologin.189.cn:9031/map/clientXML", strings.NewReader(xmlPayload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/xml")
	req.Header.Set("User-Agent", "CtClient;10.4.1;Android;13;ExampleClient")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("获取 ticket 请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// 正则匹配 Ticket
	re := regexp.MustCompile(`<Ticket>(.*?)</Ticket>`)
	matches := re.FindStringSubmatch(string(body))
	if len(matches) < 2 {
		return "", errors.New("未能在响应中找到 Ticket")
	}

	decryptedTicket, err := Decrypt(matches[1])
	if err != nil {
		return "", fmt.Errorf("Ticket 解密失败: %v", err)
	}
	return decryptedTicket, nil
}
