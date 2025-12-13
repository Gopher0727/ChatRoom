package consistenthash

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"slices"
	"sort"
	"sync"
)

// Hash 定义哈希函数接口
type Hash func(data []byte) uint32

// Ring 表示一致性哈希环
type Ring struct {
	mu       sync.RWMutex
	hash     Hash
	replicas int               // 虚拟节点数量
	keys     []uint32          // 排序的哈希环位置
	hashMap  map[uint32]string // 哈希值到节点的映射
	nodes    map[string]bool   // 存储所有真实节点
}

// New 创建一个新的一致性哈希环
// replicas: 每个真实节点对应的虚拟节点数量
// fn: 自定义哈希函数，如果为 nil 则使用默认的 SHA256
func New(replicas int, fn Hash) *Ring {
	r := &Ring{
		replicas: replicas,
		hash:     fn,
		hashMap:  make(map[uint32]string),
		nodes:    make(map[string]bool),
	}
	if r.hash == nil {
		r.hash = defaultHash
	}
	if r.replicas <= 0 {
		r.replicas = 50 // 默认虚拟节点数
	}
	return r
}

// defaultHash 默认哈希函数，使用 SHA256
func defaultHash(data []byte) uint32 {
	hash := sha256.Sum256(data)
	return binary.BigEndian.Uint32(hash[:4])
}

// Add 添加节点到哈希环
func (r *Ring) Add(nodes ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, node := range nodes {
		if node == "" {
			continue
		}

		// 如果节点已存在，跳过
		if r.nodes[node] {
			continue
		}

		r.nodes[node] = true

		// 为每个真实节点创建多个虚拟节点
		for i := 0; i < r.replicas; i++ {
			// 生成虚拟节点的键
			virtualKey := fmt.Sprintf("%s#%d", node, i)
			hash := r.hash([]byte(virtualKey))

			r.keys = append(r.keys, hash)
			r.hashMap[hash] = node
		}
	}
	slices.Sort(r.keys)
}

// Remove 从哈希环中移除节点
func (r *Ring) Remove(nodes ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, node := range nodes {
		if node == "" {
			continue
		}

		// 如果节点不存在，跳过
		if !r.nodes[node] {
			continue
		}

		delete(r.nodes, node)

		// 移除该节点的所有虚拟节点
		for i := 0; i < r.replicas; i++ {
			virtualKey := fmt.Sprintf("%s#%d", node, i)
			hash := r.hash([]byte(virtualKey))
			delete(r.hashMap, hash)
		}
	}

	// 重建 keys 切片
	r.rebuildKeys()
}

// rebuildKeys 重建排序的哈希环位置切片
func (r *Ring) rebuildKeys() {
	r.keys = r.keys[:0]
	for k := range r.hashMap {
		r.keys = append(r.keys, k)
	}
	slices.Sort(r.keys)
}

// Get 根据键获取对应的节点
// 返回顺时针方向最近的节点
func (r *Ring) Get(key string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.keys) == 0 {
		return ""
	}

	hash := r.hash([]byte(key))

	// 使用二分查找找到第一个大于等于 hash 的位置
	idx := sort.Search(len(r.keys), func(i int) bool {
		return r.keys[i] >= hash
	})

	// 如果没找到，说明 hash 大于所有节点，返回第一个节点（环形）
	if idx == len(r.keys) {
		idx = 0
	}

	return r.hashMap[r.keys[idx]]
}

// GetN 获取 key 对应的 N 个不同的节点
// 用于数据复制等场景
func (r *Ring) GetN(key string, n int) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.nodes) == 0 {
		return nil
	}

	if n > len(r.nodes) {
		n = len(r.nodes)
	}

	hash := r.hash([]byte(key))

	// 找到起始位置
	idx := sort.Search(len(r.keys), func(i int) bool {
		return r.keys[i] >= hash
	})

	result := make([]string, 0, n)
	seen := make(map[string]bool)

	// 从起始位置开始，顺时针查找 n 个不同的真实节点
	for i := 0; i < len(r.keys) && len(result) < n; i++ {
		pos := (idx + i) % len(r.keys)
		node := r.hashMap[r.keys[pos]]

		if !seen[node] {
			seen[node] = true
			result = append(result, node)
		}
	}

	return result
}

// Nodes 返回所有真实节点列表
func (r *Ring) Nodes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	nodes := make([]string, 0, len(r.nodes))
	for node := range r.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// IsEmpty 检查哈希环是否为空
func (r *Ring) IsEmpty() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.nodes) == 0
}

// Size 返回真实节点数量
func (r *Ring) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.nodes)
}
