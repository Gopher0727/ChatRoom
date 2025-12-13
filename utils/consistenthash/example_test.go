package consistenthash_test

import (
	"fmt"

	"github.com/Gopher0727/ChatRoom/utils/consistenthash"
)

// Example_basic 演示一致性哈希的基本使用
func Example_basic() {
	// 创建一致性哈希环，每个节点有 3 个虚拟节点
	ring := consistenthash.New(3, nil)

	// 添加节点
	ring.Add("node1", "node2", "node3")

	// 查找 key 对应的节点
	node := ring.Get("user:1001")
	fmt.Printf("user:1001 -> %s\n", node)

	// 同一个 key 总是映射到同一个节点
	node2 := ring.Get("user:1001")
	fmt.Printf("user:1001 -> %s (consistent)\n", node2)
}

// Example_addRemoveNodes 演示动态添加和移除节点
func Example_addRemoveNodes() {
	ring := consistenthash.New(10, nil)

	// 初始添加 3 个节点
	ring.Add("node1", "node2", "node3")
	fmt.Printf("Initial nodes: %d\n", ring.Size())

	// 添加新节点
	ring.Add("node4")
	fmt.Printf("After adding node4: %d\n", ring.Size())

	// 移除节点
	ring.Remove("node2")
	fmt.Printf("After removing node2: %d\n", ring.Size())

	// Output:
	// Initial nodes: 3
	// After adding node4: 4
	// After removing node2: 3
}

// Example_getMultipleNodes 演示获取多个节点（用于数据复制）
func Example_getMultipleNodes() {
	ring := consistenthash.New(10, nil)
	ring.Add("node1", "node2", "node3", "node4", "node5")

	// 获取 key 对应的 3 个节点（用于数据复制）
	nodes := ring.GetN("important-data", 3)
	fmt.Printf("Replicate to %d nodes\n", len(nodes))
	for i, node := range nodes {
		fmt.Printf("Replica %d: %s\n", i+1, node)
	}
}

// Example_loadBalancing 演示负载均衡场景
func Example_loadBalancing() {
	// 创建哈希环，使用较多的虚拟节点以获得更好的分布
	ring := consistenthash.New(150, nil)

	// 添加 Gateway 节点
	ring.Add("gateway1:8080", "gateway2:8080", "gateway3:8080")

	// 根据用户 ID 选择 Gateway
	userID := "user123"
	gateway := ring.Get(userID)
	fmt.Printf("User %s connects to %s\n", userID, gateway)

	// 不同的用户会被分配到不同的 Gateway
	for i := 1; i <= 5; i++ {
		uid := fmt.Sprintf("user%d", i)
		gw := ring.Get(uid)
		fmt.Printf("User %s -> %s\n", uid, gw)
	}
}

// Example_nodeFailover 演示节点故障转移
func Example_nodeFailover() {
	ring := consistenthash.New(50, nil)
	ring.Add("server1", "server2", "server3")

	// 用户连接到某个服务器
	userID := "user456"
	server := ring.Get(userID)
	fmt.Printf("User %s initially on %s\n", userID, server)

	// 模拟 server2 故障，移除节点
	ring.Remove("server2")

	// 用户会被重新分配到其他服务器
	newServer := ring.Get(userID)
	fmt.Printf("After server2 failure, user %s on %s\n", userID, newServer)

	// 如果用户原本不在 server2，则不受影响
	if server != "server2" && server == newServer {
		fmt.Println("User not affected by server2 failure")
	}
}

// Example_customHashFunction 演示使用自定义哈希函数
func Example_customHashFunction() {
	// 定义简单的哈希函数（实际使用中应该使用更好的哈希函数）
	simpleHash := func(data []byte) uint32 {
		var hash uint32
		for _, b := range data {
			hash = hash*31 + uint32(b)
		}
		return hash
	}

	ring := consistenthash.New(10, simpleHash)
	ring.Add("node1", "node2", "node3")

	node := ring.Get("test-key")
	fmt.Printf("Using custom hash: test-key -> %s\n", node)
}
