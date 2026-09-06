package xiaohongshu

// The XYW envelope is compatible with the MIT-licensed xhshow project:
// https://github.com/Cloxl/xhshow (Copyright 2024 Cloxl).

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
)

const (
	xhs_xyw_key       = "7cc4adla5ay0701v"
	xhs_xyw_iv        = "4uzjr7mbsibcaldp"
	xhs_xyw_env_flags = "0|0|0|1|0|0|1|0|0|0|1|0|0|0|0|1|0|0|1"
)

type xhs_xyw_envelope struct {
	SignSVN     string `json:"signSvn"`
	SignType    string `json:"signType"`
	AppID       string `json:"appId"`
	SignVersion string `json:"signVersion"`
	Payload     string `json:"payload"`
}

func xhs_pkcs7_pad(value []byte, block_size int) []byte {
	padding := block_size - len(value)%block_size
	result := make([]byte, len(value)+padding)
	copy(result, value)
	for index := len(value); index < len(result); index++ {
		result[index] = byte(padding)
	}
	return result
}

func xhs_xyw_signature(full_uri string, a1_value string, timestamp_ms int64) (string, error) {
	digest := md5.Sum([]byte("url=" + full_uri))
	message := "x1=" + hex.EncodeToString(digest[:]) +
		";x2=" + xhs_xyw_env_flags +
		";x3=" + a1_value +
		";x4=" + strconv.FormatInt(timestamp_ms, 10) + ";"
	plaintext := xhs_pkcs7_pad([]byte(base64.StdEncoding.EncodeToString([]byte(message))), aes.BlockSize)
	block, err := aes.NewCipher([]byte(xhs_xyw_key))
	if err != nil {
		return "", fmt.Errorf("xiaohongshu XYW cipher: %w", err)
	}
	ciphertext := make([]byte, len(plaintext))
	cipher.NewCBCEncrypter(block, []byte(xhs_xyw_iv)).CryptBlocks(ciphertext, plaintext)
	envelope_data, err := json.Marshal(xhs_xyw_envelope{
		SignSVN: "56", SignType: "x2", AppID: "xhs-pc-web", SignVersion: "1",
		Payload: hex.EncodeToString(ciphertext),
	})
	if err != nil {
		return "", fmt.Errorf("xiaohongshu XYW envelope: %w", err)
	}
	return "XYW_" + base64.StdEncoding.EncodeToString(envelope_data), nil
}
