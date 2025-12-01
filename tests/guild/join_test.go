package guild_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	BaseURL    = "http://localhost:9000/api/v1"
	UserCount  = 2000 // 模拟的用户数量
	WorkerPool = 200  // 降低并发数，避免单机端口耗尽
)

// 全局 HTTP 客户端，复用连接
var httpClient *http.Client

func init() {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 1000
	t.MaxIdleConnsPerHost = 1000
	t.IdleConnTimeout = 90 * time.Second

	httpClient = &http.Client{
		Transport: t,
		Timeout:   100 * time.Second,
	}
}

// 响应结构体
type LoginResponse struct {
	Token string `json:"token"`
}

type CreateGuildResponse struct {
	ID uint `json:"id"`
}

type CreateInviteResponse struct {
	Code string `json:"code"`
}

func TestJoin(t *testing.T) {
	// 使用局部随机数生成器，避免全局锁竞争，并确保随机性
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	t.Log("=== 开始批量加入 Guild 测试 ===")

	// 1. 准备 Owner 用户并创建 Guild 和邀请码
	ownerToken := setupOwner(t, rng)
	guildID := createGuild(t, ownerToken)
	inviteCode := createInvite(t, ownerToken, guildID)

	t.Logf("准备就绪: GuildID=%d, InviteCode=%s\n", guildID, inviteCode)
	t.Logf("即将模拟 %d 个用户并发加入...\n", UserCount)

	// 2. 并发注册、登录并加入
	var successCount int32
	var failCount int32
	var wg sync.WaitGroup

	// 使用 channel 控制并发数（可选，防止本地端口耗尽）
	sem := make(chan struct{}, WorkerPool)

	startTime := time.Now()

	for i := range UserCount {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}        // 获取令牌
			defer func() { <-sem }() // 释放令牌

			// 生成符合 validator 规则的唯一用户名 (3-20字符, 字母数字下划线)
			// 格式: u_{timestamp_suffix}_{idx}
			// 例如: u_123456_1
			timestampSuffix := time.Now().UnixNano() % 1000000
			username := fmt.Sprintf("u_%d_%d", timestampSuffix, idx)
			email := fmt.Sprintf("u_%d_%d@test.com", timestampSuffix, idx)
			password := "password123" // 符合 >= 8 字符

			// 注册
			if err := register(username, email, password); err != nil {
				t.Logf("[User %d] 注册失败: %v", idx, err)
				atomic.AddInt32(&failCount, 1)
				return
			}

			// 登录
			token, err := login(username, password)
			if err != nil {
				t.Logf("[User %d] 登录失败: %v", idx, err)
				atomic.AddInt32(&failCount, 1)
				return
			}

			// 加入 Guild
			if err := joinGuild(token, inviteCode); err != nil {
				t.Logf("[User %d] 加入失败: %v", idx, err)
				atomic.AddInt32(&failCount, 1)
				return
			}

			t.Logf("[User %d] 加入成功", idx)
			atomic.AddInt32(&successCount, 1)
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	t.Log("=== 测试结束 ===")
	t.Logf("总耗时: %v\n", duration)
	t.Logf("成功: %d\n", successCount)
	t.Logf("失败: %d\n", failCount)

	if failCount > 0 {
		t.Errorf("测试失败: 有 %d 个用户未能成功加入", failCount)
	}
}

// --- 辅助函数 ---

func setupOwner(t *testing.T, rng *rand.Rand) string {
	// 生成唯一的 Owner 用户名
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

func createGuild(t *testing.T, token string) uint {
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

func createInvite(t *testing.T, token string, guildID uint) string {
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

	// 使用全局 httpClient 复用连接
	resp, err := httpClient.Do(req)
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
