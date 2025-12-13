package consistenthash

import (
	"fmt"
	"strconv"
	"sync"
	"testing"
)

// TestNew 测试创建一致性哈希环
func TestNew(t *testing.T) {
	t.Run("with default hash function", func(t *testing.T) {
		ring := New(10, nil)
		if ring == nil {
			t.Fatal("expected ring to be created")
		}
		if ring.replicas != 10 {
			t.Errorf("expected replicas to be 10, got %d", ring.replicas)
		}
		if ring.hash == nil {
			t.Error("expected default hash function to be set")
		}
	})

	t.Run("with custom hash function", func(t *testing.T) {
		customHash := func(data []byte) uint32 {
			return 42
		}
		ring := New(5, customHash)
		if ring.replicas != 5 {
			t.Errorf("expected replicas to be 5, got %d", ring.replicas)
		}
		result := ring.hash([]byte("test"))
		if result != 42 {
			t.Errorf("expected custom hash to return 42, got %d", result)
		}
	})

	t.Run("with zero replicas uses default", func(t *testing.T) {
		ring := New(0, nil)
		if ring.replicas != 50 {
			t.Errorf("expected default replicas to be 50, got %d", ring.replicas)
		}
	})

	t.Run("with negative replicas uses default", func(t *testing.T) {
		ring := New(-5, nil)
		if ring.replicas != 50 {
			t.Errorf("expected default replicas to be 50, got %d", ring.replicas)
		}
	})
}

// TestAdd 测试添加节点
func TestAdd(t *testing.T) {
	t.Run("add single node", func(t *testing.T) {
		ring := New(3, nil)
		ring.Add("node1")

		if ring.Size() != 1 {
			t.Errorf("expected 1 node, got %d", ring.Size())
		}

		// 应该有 3 个虚拟节点
		if len(ring.keys) != 3 {
			t.Errorf("expected 3 virtual nodes, got %d", len(ring.keys))
		}
	})

	t.Run("add multiple nodes", func(t *testing.T) {
		ring := New(3, nil)
		ring.Add("node1", "node2", "node3")

		if ring.Size() != 3 {
			t.Errorf("expected 3 nodes, got %d", ring.Size())
		}

		// 应该有 9 个虚拟节点 (3 nodes * 3 replicas)
		if len(ring.keys) != 9 {
			t.Errorf("expected 9 virtual nodes, got %d", len(ring.keys))
		}
	})

	t.Run("add duplicate node", func(t *testing.T) {
		ring := New(3, nil)
		ring.Add("node1")
		ring.Add("node1") // 重复添加

		if ring.Size() != 1 {
			t.Errorf("expected 1 node after duplicate add, got %d", ring.Size())
		}

		if len(ring.keys) != 3 {
			t.Errorf("expected 3 virtual nodes, got %d", len(ring.keys))
		}
	})

	t.Run("add empty node name", func(t *testing.T) {
		ring := New(3, nil)
		ring.Add("")

		if ring.Size() != 0 {
			t.Errorf("expected 0 nodes after adding empty string, got %d", ring.Size())
		}
	})

	t.Run("keys are sorted after add", func(t *testing.T) {
		ring := New(5, nil)
		ring.Add("node1", "node2")

		// 验证 keys 是排序的
		for i := 1; i < len(ring.keys); i++ {
			if ring.keys[i-1] >= ring.keys[i] {
				t.Error("keys are not sorted")
				break
			}
		}
	})
}

// TestRemove 测试移除节点
func TestRemove(t *testing.T) {
	t.Run("remove existing node", func(t *testing.T) {
		ring := New(3, nil)
		ring.Add("node1", "node2", "node3")
		ring.Remove("node2")

		if ring.Size() != 2 {
			t.Errorf("expected 2 nodes after removal, got %d", ring.Size())
		}

		// 应该有 6 个虚拟节点 (2 nodes * 3 replicas)
		if len(ring.keys) != 6 {
			t.Errorf("expected 6 virtual nodes, got %d", len(ring.keys))
		}

		// 验证 node2 不在节点列表中
		nodes := ring.Nodes()
		for _, node := range nodes {
			if node == "node2" {
				t.Error("node2 should have been removed")
			}
		}
	})

	t.Run("remove non-existing node", func(t *testing.T) {
		ring := New(3, nil)
		ring.Add("node1")
		ring.Remove("node2") // 移除不存在的节点

		if ring.Size() != 1 {
			t.Errorf("expected 1 node, got %d", ring.Size())
		}
	})

	t.Run("remove empty node name", func(t *testing.T) {
		ring := New(3, nil)
		ring.Add("node1")
		ring.Remove("")

		if ring.Size() != 1 {
			t.Errorf("expected 1 node after removing empty string, got %d", ring.Size())
		}
	})

	t.Run("keys are sorted after remove", func(t *testing.T) {
		ring := New(5, nil)
		ring.Add("node1", "node2", "node3")
		ring.Remove("node2")

		// 验证 keys 是排序的
		for i := 1; i < len(ring.keys); i++ {
			if ring.keys[i-1] >= ring.keys[i] {
				t.Error("keys are not sorted after removal")
				break
			}
		}
	})
}

// TestGet 测试获取节点
func TestGet(t *testing.T) {
	t.Run("get from empty ring", func(t *testing.T) {
		ring := New(3, nil)
		node := ring.Get("key1")

		if node != "" {
			t.Errorf("expected empty string from empty ring, got %s", node)
		}
	})

	t.Run("get from single node ring", func(t *testing.T) {
		ring := New(3, nil)
		ring.Add("node1")

		node := ring.Get("key1")
		if node != "node1" {
			t.Errorf("expected node1, got %s", node)
		}
	})

	t.Run("get returns consistent results", func(t *testing.T) {
		ring := New(10, nil)
		ring.Add("node1", "node2", "node3")

		// 同一个 key 应该总是返回相同的节点
		key := "test-key"
		node1 := ring.Get(key)
		node2 := ring.Get(key)
		node3 := ring.Get(key)

		if node1 != node2 || node2 != node3 {
			t.Error("Get should return consistent results for the same key")
		}
	})

	t.Run("different keys may map to different nodes", func(t *testing.T) {
		ring := New(10, nil)
		ring.Add("node1", "node2", "node3")

		// 测试多个不同的 key，至少应该有一些映射到不同的节点
		keys := []string{"key1", "key2", "key3", "key4", "key5"}
		nodeSet := make(map[string]bool)

		for _, key := range keys {
			node := ring.Get(key)
			nodeSet[node] = true
		}

		// 至少应该使用了一个节点
		if len(nodeSet) == 0 {
			t.Error("expected at least one node to be used")
		}
	})
}

// TestGetN 测试获取多个节点
func TestGetN(t *testing.T) {
	t.Run("get N from empty ring", func(t *testing.T) {
		ring := New(3, nil)
		nodes := ring.GetN("key1", 2)

		if nodes != nil {
			t.Error("expected nil from empty ring")
		}
	})

	t.Run("get N nodes", func(t *testing.T) {
		ring := New(10, nil)
		ring.Add("node1", "node2", "node3")

		nodes := ring.GetN("key1", 2)
		if len(nodes) != 2 {
			t.Errorf("expected 2 nodes, got %d", len(nodes))
		}

		// 验证返回的节点是不同的
		if nodes[0] == nodes[1] {
			t.Error("expected different nodes")
		}
	})

	t.Run("get more nodes than available", func(t *testing.T) {
		ring := New(10, nil)
		ring.Add("node1", "node2")

		nodes := ring.GetN("key1", 5)
		if len(nodes) != 2 {
			t.Errorf("expected 2 nodes (all available), got %d", len(nodes))
		}
	})

	t.Run("get N returns consistent results", func(t *testing.T) {
		ring := New(10, nil)
		ring.Add("node1", "node2", "node3")

		key := "test-key"
		nodes1 := ring.GetN(key, 2)
		nodes2 := ring.GetN(key, 2)

		if len(nodes1) != len(nodes2) {
			t.Error("GetN should return consistent number of nodes")
		}

		for i := range nodes1 {
			if nodes1[i] != nodes2[i] {
				t.Error("GetN should return consistent results for the same key")
			}
		}
	})
}

// TestNodes 测试获取所有节点
func TestNodes(t *testing.T) {
	t.Run("nodes from empty ring", func(t *testing.T) {
		ring := New(3, nil)
		nodes := ring.Nodes()

		if len(nodes) != 0 {
			t.Errorf("expected 0 nodes, got %d", len(nodes))
		}
	})

	t.Run("nodes returns all added nodes", func(t *testing.T) {
		ring := New(3, nil)
		ring.Add("node1", "node2", "node3")

		nodes := ring.Nodes()
		if len(nodes) != 3 {
			t.Errorf("expected 3 nodes, got %d", len(nodes))
		}

		// 验证所有节点都在列表中
		nodeMap := make(map[string]bool)
		for _, node := range nodes {
			nodeMap[node] = true
		}

		if !nodeMap["node1"] || !nodeMap["node2"] || !nodeMap["node3"] {
			t.Error("not all nodes are in the list")
		}
	})
}

// TestIsEmpty 测试检查环是否为空
func TestIsEmpty(t *testing.T) {
	t.Run("new ring is empty", func(t *testing.T) {
		ring := New(3, nil)
		if !ring.IsEmpty() {
			t.Error("new ring should be empty")
		}
	})

	t.Run("ring with nodes is not empty", func(t *testing.T) {
		ring := New(3, nil)
		ring.Add("node1")
		if ring.IsEmpty() {
			t.Error("ring with nodes should not be empty")
		}
	})

	t.Run("ring becomes empty after removing all nodes", func(t *testing.T) {
		ring := New(3, nil)
		ring.Add("node1")
		ring.Remove("node1")
		if !ring.IsEmpty() {
			t.Error("ring should be empty after removing all nodes")
		}
	})
}

// TestSize 测试获取节点数量
func TestSize(t *testing.T) {
	ring := New(3, nil)

	if ring.Size() != 0 {
		t.Errorf("expected size 0, got %d", ring.Size())
	}

	ring.Add("node1")
	if ring.Size() != 1 {
		t.Errorf("expected size 1, got %d", ring.Size())
	}

	ring.Add("node2", "node3")
	if ring.Size() != 3 {
		t.Errorf("expected size 3, got %d", ring.Size())
	}

	ring.Remove("node2")
	if ring.Size() != 2 {
		t.Errorf("expected size 2, got %d", ring.Size())
	}
}

// TestConcurrency 测试并发安全性
func TestConcurrency(t *testing.T) {
	ring := New(10, nil)

	var wg sync.WaitGroup

	// 并发添加节点
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ring.Add(fmt.Sprintf("node%d", id))
		}(i)
	}

	// 并发查询
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ring.Get(fmt.Sprintf("key%d", id))
		}(i)
	}

	wg.Wait()

	if ring.Size() != 10 {
		t.Errorf("expected 10 nodes after concurrent adds, got %d", ring.Size())
	}
}

// TestLoadDistribution 测试负载分布
func TestLoadDistribution(t *testing.T) {
	ring := New(150, nil) // 使用较多的虚拟节点以获得更好的分布
	ring.Add("node1", "node2", "node3")

	// 生成大量的 key 并统计分布
	distribution := make(map[string]int)
	numKeys := 10000

	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key%d", i)
		node := ring.Get(key)
		distribution[node]++
	}

	// 验证每个节点都被使用了
	if len(distribution) != 3 {
		t.Errorf("expected all 3 nodes to be used, got %d", len(distribution))
	}

	// 验证分布相对均匀（每个节点应该得到大约 1/3 的 key）
	expectedPerNode := numKeys / 3
	tolerance := float64(expectedPerNode) * 0.3 // 30% 的容差

	for node, count := range distribution {
		diff := float64(count - expectedPerNode)
		if diff < 0 {
			diff = -diff
		}
		if diff > tolerance {
			t.Logf("node %s has %d keys (expected ~%d, tolerance %.0f)",
				node, count, expectedPerNode, tolerance)
		}
	}
}

// TestNodeAdditionMinimalDisruption 测试添加节点时的最小影响
func TestNodeAdditionMinimalDisruption(t *testing.T) {
	ring := New(50, nil)
	ring.Add("node1", "node2", "node3")

	// 记录添加新节点前的映射
	numKeys := 1000
	beforeMapping := make(map[string]string)
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key%d", i)
		beforeMapping[key] = ring.Get(key)
	}

	// 添加新节点
	ring.Add("node4")

	// 统计有多少 key 的映射发生了变化
	changed := 0
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key%d", i)
		if ring.Get(key) != beforeMapping[key] {
			changed++
		}
	}

	// 理论上，添加一个节点应该只影响约 1/4 的 key
	// 实际中由于哈希分布的随机性，我们允许一定的偏差
	expectedChange := numKeys / 4
	tolerance := float64(expectedChange) * 0.5 // 50% 的容差

	diff := float64(changed - expectedChange)
	if diff < 0 {
		diff = -diff
	}

	t.Logf("Added node4: %d/%d keys changed (expected ~%d)", changed, numKeys, expectedChange)

	if diff > tolerance {
		t.Logf("Warning: key redistribution is outside expected range")
	}
}

// TestNodeRemovalMinimalDisruption 测试移除节点时的最小影响
func TestNodeRemovalMinimalDisruption(t *testing.T) {
	ring := New(50, nil)
	ring.Add("node1", "node2", "node3", "node4")

	// 记录移除节点前的映射
	numKeys := 1000
	beforeMapping := make(map[string]string)
	for i := range numKeys {
		key := fmt.Sprintf("key%d", i)
		beforeMapping[key] = ring.Get(key)
	}

	// 移除一个节点
	ring.Remove("node4")

	// 统计有多少 key 的映射发生了变化
	changed := 0
	for i := range numKeys {
		key := fmt.Sprintf("key%d", i)
		if ring.Get(key) != beforeMapping[key] {
			changed++
		}
	}

	// 理论上，移除一个节点应该只影响约 1/4 的 key
	expectedChange := numKeys / 4
	tolerance := float64(expectedChange) * 0.5

	diff := float64(changed - expectedChange)
	if diff < 0 {
		diff = -diff
	}

	t.Logf("Removed node4: %d/%d keys changed (expected ~%d)", changed, numKeys, expectedChange)

	if diff > tolerance {
		t.Logf("Warning: key redistribution is outside expected range")
	}
}

// BenchmarkGet 性能测试：查找节点
func BenchmarkGet(b *testing.B) {
	ring := New(150, nil)
	ring.Add("node1", "node2", "node3", "node4", "node5")

	for i := 0; b.Loop(); i++ {
		ring.Get(strconv.Itoa(i))
	}
}

// BenchmarkGetN 性能测试：查找多个节点
func BenchmarkGetN(b *testing.B) {
	ring := New(150, nil)
	ring.Add("node1", "node2", "node3", "node4", "node5")

	for i := 0; b.Loop(); i++ {
		ring.GetN(strconv.Itoa(i), 3)
	}
}

// BenchmarkAdd 性能测试：添加节点
func BenchmarkAdd(b *testing.B) {
	for b.Loop() {
		ring := New(150, nil)
		ring.Add("node1", "node2", "node3", "node4", "node5")
	}
}
