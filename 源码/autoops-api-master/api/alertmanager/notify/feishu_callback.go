package notify

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"dodevops-api/api/alertmanager/model"
)

type FeishuEventEnvelope struct {
	Type   string `json:"type"`
	Schema string `json:"schema"`
	Token  string `json:"token"`
	Header struct {
		EventID   string `json:"event_id"`
		EventType string `json:"event_type"`
		Token     string `json:"token"`
		AppID     string `json:"app_id"`
		TenantKey string `json:"tenant_key"`
	} `json:"header"`
	Event struct {
		Message struct {
			ChatID   string `json:"chat_id"`
			ChatType string `json:"chat_type"`
		} `json:"message"`
	} `json:"event"`
	Challenge string `json:"challenge"`
	Encrypt   string `json:"encrypt"`
}

func ParseFeishuEventPayload(raw []byte, encryptKey string) (*FeishuEventEnvelope, error) {
	plain, err := DecodeFeishuEventPayload(raw, encryptKey)
	if err != nil {
		return nil, err
	}
	var envelope FeishuEventEnvelope
	if err := json.Unmarshal(plain, &envelope); err != nil {
		return nil, err
	}
	return &envelope, nil
}

func DecodeFeishuEventPayload(raw []byte, encryptKey string) ([]byte, error) {
	var envelope FeishuEventEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	if strings.TrimSpace(envelope.Encrypt) == "" {
		return append([]byte(nil), raw...), nil
	}
	if strings.TrimSpace(encryptKey) == "" {
		return nil, errors.New("encryptKey is required")
	}
	plain, err := decryptFeishuEvent(envelope.Encrypt, encryptKey)
	if err != nil {
		return nil, err
	}
	return []byte(plain), nil
}

func FeishuEventPayloadEncrypted(raw []byte) (bool, error) {
	var envelope FeishuEventEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return false, err
	}
	return strings.TrimSpace(envelope.Encrypt) != "", nil
}

func VerifyFeishuEventSignature(headers http.Header, raw []byte, encryptKey string) error {
	if strings.TrimSpace(encryptKey) == "" {
		return nil
	}
	timestamp := strings.TrimSpace(headers.Get("X-Lark-Request-Timestamp"))
	nonce := strings.TrimSpace(headers.Get("X-Lark-Request-Nonce"))
	signature := strings.TrimSpace(headers.Get("X-Lark-Signature"))
	if timestamp == "" || nonce == "" || signature == "" {
		return errors.New("missing feishu signature headers")
	}
	sum := sha256.Sum256(append([]byte(timestamp+nonce+encryptKey), raw...))
	if strings.TrimSpace(signature) != hex.EncodeToString(sum[:]) {
		return errors.New("invalid feishu signature")
	}
	return nil
}

func MatchFeishuCardChannelByAppID(channels []model.AlertNotifyChannel, appID string) *model.AlertNotifyChannel {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil
	}
	for i := range channels {
		if strings.TrimSpace(channels[i].Type) != model.NotifyChannelFeishuCard {
			continue
		}
		cfg, err := parseConfig(channels[i].Config)
		if err != nil {
			continue
		}
		if strings.TrimSpace(cfg.AppID) == appID {
			return &channels[i]
		}
	}
	return nil
}

func decryptFeishuEvent(cipherText, encryptKey string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return "", err
	}
	if len(decoded) < aes.BlockSize {
		return "", errors.New("invalid encrypted payload")
	}
	key := sha256.Sum256([]byte(encryptKey))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	iv := decoded[:aes.BlockSize]
	body := decoded[aes.BlockSize:]
	if len(body) == 0 || len(body)%aes.BlockSize != 0 {
		return "", errors.New("invalid encrypted body length")
	}
	plain := make([]byte, len(body))
	mode := newCBCDecrypter(block, iv)
	mode.CryptBlocks(plain, body)
	plain = pkcs7Unpad(plain)
	if len(plain) == 0 {
		return "", fmt.Errorf("decrypt payload empty")
	}
	return string(plain), nil
}

type cbcDecrypter interface {
	CryptBlocks(dst, src []byte)
}

func newCBCDecrypter(block cipher.Block, iv []byte) cbcDecrypter {
	return cbcDecrypterWrapper{block: block, iv: append([]byte(nil), iv...)}
}

type cbcDecrypterWrapper struct {
	block cipher.Block
	iv    []byte
}

func (c cbcDecrypterWrapper) CryptBlocks(dst, src []byte) {
	if len(src)%aes.BlockSize != 0 {
		return
	}
	prev := append([]byte(nil), c.iv...)
	for i := 0; i < len(src); i += aes.BlockSize {
		c.block.Decrypt(dst[i:i+aes.BlockSize], src[i:i+aes.BlockSize])
		for j := 0; j < aes.BlockSize; j++ {
			dst[i+j] ^= prev[j]
		}
		copy(prev, src[i:i+aes.BlockSize])
	}
}

func pkcs7Unpad(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	padding := int(data[len(data)-1])
	if padding <= 0 || padding > len(data) {
		return data
	}
	return data[:len(data)-padding]
}
