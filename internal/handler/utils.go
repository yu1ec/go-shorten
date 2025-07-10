package handler

import (
	"crypto/rand"
	"math/big"
)

// 用于生成随机短代码的字符集
const Charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// GenerateRandomCode 生成指定长度的随机短代码
func GenerateRandomCode(length int) (string, error) {
	b := make([]byte, length)
	for i := range b {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(Charset))))
		if err != nil {
			return "", err
		}
		b[i] = Charset[num.Int64()]
	}
	return string(b), nil
}
