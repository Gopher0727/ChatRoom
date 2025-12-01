package guild

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestJoin(t *testing.T) {
	t.Log("=== 开始批量加入 Guild 测试 ===")

	// 准备 Owner 用户并创建 Guild 和邀请码
	ownerToken := CreateUser(t)
	guildID := CreateGuild(t, ownerToken)
	inviteCode := CreateInvite(t, ownerToken, guildID)

	t.Logf("准备就绪: GuildID=%d, InviteCode=%s\n", guildID, inviteCode)
	t.Logf("即将模拟 %d 个用户并发加入...\n", UserCount)

	// 并发注册、登录并加入
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
