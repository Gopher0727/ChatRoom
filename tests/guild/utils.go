package guild

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"testing"
	"time"
)

const (
	BaseURL    = "http://localhost:9000/api/v1"
	UserCount  = 2000 // 模拟的用户数量
	WorkerPool = 200  // 降低并发数，避免单机端口耗尽
)

var HttpClient *http.Client

func init() {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 1000
	t.MaxIdleConnsPerHost = 1000
	t.IdleConnTimeout = 90 * time.Second

	HttpClient = &http.Client{
		Transport: t,
		Timeout:   100 * time.Second,
	}
}

type LoginResponse struct {
	Token string `json:"token"`
}

type CreateGuildResponse struct {
	ID uint `json:"id"`
}

type CreateInviteResponse struct {
	Code string `json:"code"`
}

type SendMessageRequest struct {
	Content string `json:"content"`
}

type MessageResponse struct {
	ID         int64     `json:"id"`
	Content    string    `json:"content"`
	SequenceID int64     `json:"sequence_id"`
	CreatedAt  time.Time `json:"created_at"`
}

type GuildWithUnread struct {
	ID          uint      `json:"id"`
	Topic       string    `json:"topic"`
	OwnerID     uint      `json:"owner_id"`
	CreatedAt   time.Time `json:"created_at"`
	UnreadCount int64     `json:"unread_count"`
}

func CreateUser(t *testing.T) string {
	// 使用局部随机数生成器，避免全局锁竞争，并确保随机性
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	suffix := rng.Intn(100000)

	username := fmt.Sprintf("owner_%d", suffix)
	email := fmt.Sprintf("owner_%d@test.com", suffix)
	password := "password123"

	if err := register(username, email, password); err != nil {
		t.Fatalf("Owner 注册失败: %v", err)
	}
	token, err := login(username, password)
	if err != nil {
		t.Fatalf("Owner 登录失败: %v", err)
	}
	return token
}

func CreateGuild(t *testing.T, token string) uint {
	data := map[string]string{"topic": "Batch Test Guild"}
	body, _ := json.Marshal(data)
	resp, err := sendRequest("POST", BaseURL+"/guilds", token, body)
	if err != nil {
		t.Fatalf("创建 Guild 失败: %v", err)
	}
	var res CreateGuildResponse
	json.Unmarshal(resp, &res)
	return res.ID
}

func CreateInvite(t *testing.T, token string, guildID uint) string {
	resp, err := sendRequest("POST", fmt.Sprintf("%s/guilds/%d/invites", BaseURL, guildID), token, nil)
	if err != nil {
		t.Fatalf("创建邀请码失败: %v", err)
	}
	var res CreateInviteResponse
	json.Unmarshal(resp, &res)
	return res.Code
}

func register(username, email, password string) error {
	data := map[string]string{
		"username": username,
		"email":    email,
		"password": password,
	}
	body, _ := json.Marshal(data)
	_, err := sendRequest("POST", BaseURL+"/users/register", "", body)
	return err
}

func login(username, password string) (string, error) {
	data := map[string]string{
		"username": username,
		"password": password,
	}
	body, _ := json.Marshal(data)
	resp, err := sendRequest("POST", BaseURL+"/users/login", "", body)
	if err != nil {
		return "", err
	}
	var res LoginResponse
	if err := json.Unmarshal(resp, &res); err != nil {
		return "", err
	}
	if res.Token == "" {
		return "", fmt.Errorf("token is empty")
	}
	return res.Token, nil
}

func joinGuild(token, inviteCode string) error {
	data := map[string]string{"invite_code": inviteCode}
	body, _ := json.Marshal(data)
	_, err := sendRequest("POST", BaseURL+"/guilds/join", token, body)
	return err
}

func sendRequest(method, url string, token string, body []byte) ([]byte, error) {
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func sendMessage(t *testing.T, token string, guildID uint, content string) MessageResponse {
	req := SendMessageRequest{Content: content}
	body, _ := json.Marshal(req)
	resp, err := sendRequest("POST", fmt.Sprintf("%s/guilds/%d/messages", BaseURL, guildID), token, body)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	var msg MessageResponse
	if err := json.Unmarshal(resp, &msg); err != nil {
		t.Fatalf("Failed to unmarshal message response: %v", err)
	}
	return msg
}

func getMessagesAfter(t *testing.T, token string, guildID uint, afterSeq int64) []MessageResponse {
	url := fmt.Sprintf("%s/guilds/%d/messages?after_seq=%d", BaseURL, guildID, afterSeq)
	resp, err := sendRequest("GET", url, token, nil)
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	var msgs []MessageResponse
	if err := json.Unmarshal(resp, &msgs); err != nil {
		t.Fatalf("Failed to unmarshal messages response: %v", err)
	}
	return msgs
}

func getMyGuilds(t *testing.T, token string) []GuildWithUnread {
	resp, err := sendRequest("GET", BaseURL+"/guilds/mine", token, nil)
	if err != nil {
		t.Fatalf("GetMyGuilds failed: %v", err)
	}
	var guilds []GuildWithUnread
	if err := json.Unmarshal(resp, &guilds); err != nil {
		t.Fatalf("Failed to unmarshal guilds response: %v", err)
	}
	return guilds
}

func ackMessage(t *testing.T, token string, guildID uint, sequenceID int64) {
	data := map[string]int64{"sequence_id": sequenceID}
	body, _ := json.Marshal(data)
	_, err := sendRequest("POST", fmt.Sprintf("%s/guilds/%d/ack", BaseURL, guildID), token, body)
	if err != nil {
		t.Fatalf("AckMessage failed: %v", err)
	}
}
