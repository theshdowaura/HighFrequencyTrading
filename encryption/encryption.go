package encryption

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/des"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
)

// Encrypt3DES : CBC模式
func Encrypt3DES(key, iv, plaintext []byte) (string, error) {
	block, err := des.NewTripleDESCipher(key)
	if err != nil {
		return "", err
	}
	if len(iv) != block.BlockSize() {
		return "", errors.New("3des iv size error")
	}
	plaintext = pkcs7Pad(plaintext, block.BlockSize())
	mode := cipher.NewCBCEncrypter(block, iv)
	out := make([]byte, len(plaintext))
	mode.CryptBlocks(out, plaintext)
	return hex.EncodeToString(out), nil
}

// Decrypt3DES : CBC模式
func Decrypt3DES(key, iv []byte, cipherHex string) (string, error) {
	data, err := hex.DecodeString(cipherHex)
	if err != nil {
		return "", err
	}
	block, err := des.NewTripleDESCipher(key)
	if err != nil {
		return "", err
	}
	if len(iv) != block.BlockSize() || len(data)%block.BlockSize() != 0 {
		return "", errors.New("3des data or iv size error")
	}
	mode := cipher.NewCBCDecrypter(block, iv)
	out := make([]byte, len(data))
	mode.CryptBlocks(out, data)
	out, err = pkcs7Unpad(out, block.BlockSize())
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// EncryptRSA : 公钥加密 -> base64
func EncryptRSA(pubPEM string, plaintext []byte) (string, error) {
	block, _ := pem.Decode([]byte(pubPEM))
	if block == nil {
		return "", errors.New("invalid RSA public key PEM")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", err
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return "", errors.New("not RSA public key")
	}
	encrypted, err := rsa.EncryptPKCS1v15(crand.Reader, rsaPub, plaintext)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// EncryptRSAHex : 公钥加密 -> hex
func EncryptRSAHex(pubPEM string, plaintext []byte) (string, error) {
	block, _ := pem.Decode([]byte(pubPEM))
	if block == nil {
		return "", errors.New("invalid RSA public key PEM")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", err
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return "", errors.New("not RSA public key")
	}
	encrypted, err := rsa.EncryptPKCS1v15(crand.Reader, rsaPub, plaintext)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(encrypted), nil
}

// EncryptAES_ECB : AES ECB模式(不安全，但兼容需求)
func EncryptAES_ECB(key, plaintext []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	bs := block.BlockSize()
	plaintext = pkcs7Pad(plaintext, bs)
	out := make([]byte, len(plaintext))
	tmp := make([]byte, bs)
	for i := 0; i < len(plaintext); i += bs {
		block.Encrypt(tmp, plaintext[i:i+bs])
		copy(out[i:i+bs], tmp)
	}
	return hex.EncodeToString(out), nil
}

// pkcs7Pad / pkcs7Unpad
func pkcs7Pad(data []byte, blockSize int) []byte {
	padLen := blockSize - len(data)%blockSize
	padtext := bytes.Repeat([]byte{byte(padLen)}, padLen)
	return append(data, padtext...)
}
func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	length := len(data)
	if length == 0 {
		return nil, errors.New("data error")
	}
	unpad := int(data[length-1])
	if unpad > blockSize || unpad == 0 {
		return nil, errors.New("padding size error")
	}
	return data[:(length - unpad)], nil
}
