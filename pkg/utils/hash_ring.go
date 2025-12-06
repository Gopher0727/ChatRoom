package utils

import (
	"crypto/sha1"
	"encoding/binary"
	"sort"
	"strconv"
	"sync"
)

// HashRing 提供一致性哈希环，支持权重（以虚拟节点数量体现）。
type HashRing struct {
	mu       sync.RWMutex
	replicas int      // 每个节点的基础虚拟节点数
	ring     []uint32 // 有序的哈希环（按hash升序）

	nodesMap map[uint32]string // 哈希值 -> 节点名
	weights  map[string]int    // 节点名 -> 权重（额外虚拟节点系数）
}

// NewHashRing 创建哈希环，replicas 表示每个真实节点的基础虚拟节点数（推荐 100~200）。
func NewHashRing(replicas int) *HashRing {
	if replicas <= 0 {
		replicas = 128
	}
	return &HashRing{
		replicas: replicas,
		nodesMap: make(map[uint32]string),
		weights:  make(map[string]int),
	}
}

// ringHash 生成 32bit 哈希。
func ringHash(b []byte) uint32 {
	h := sha1.Sum(b)
	// 取前4字节为32位数，分布较均匀
	return binary.BigEndian.Uint32(h[:4])
}

// Add 添加一个节点，weight 为权重（>=1），影响虚拟节点数量。
func (hr *HashRing) Add(node string, weight int) {
	if weight <= 0 {
		weight = 1
	}
	hr.mu.Lock()
	defer hr.mu.Unlock()

	// 若已存在，先移除后重建，确保权重更新
	if _, ok := hr.weights[node]; ok {
		hr.removeNodeLocked(node)
	}

	hr.weights[node] = weight
	totalReplicas := hr.replicas * weight

	// 为该节点生成虚拟节点
	for i := 0; i < totalReplicas; i++ {
		// 采用 node#i 作为虚拟节点key
		key := node + "#" + strconv.Itoa(i)
		hv := ringHash([]byte(key))
		// 避免哈希冲突覆盖，若冲突则线性探测递增
		for {
			if _, exists := hr.nodesMap[hv]; !exists {
				break
			}
			hv++
		}
		hr.ring = append(hr.ring, hv)
		hr.nodesMap[hv] = node
	}
	sort.Slice(hr.ring, func(i, j int) bool { return hr.ring[i] < hr.ring[j] })
}

// Remove 删除一个节点。
func (hr *HashRing) Remove(node string) {
	hr.mu.Lock()
	defer hr.mu.Unlock()
	hr.removeNodeLocked(node)
}

// removeNodeLocked 在持有写锁的前提下移除节点的所有虚拟节点。
func (hr *HashRing) removeNodeLocked(node string) {
	if _, ok := hr.weights[node]; !ok {
		return
	}
	delete(hr.weights, node)

	// 过滤掉属于该节点的哈希值
	filtered := hr.ring[:0]
	for _, hv := range hr.ring {
		if hr.nodesMap[hv] != node {
			filtered = append(filtered, hv)
		} else {
			delete(hr.nodesMap, hv)
		}
	}
	// 重新赋值有序环
	hr.ring = make([]uint32, len(filtered))
	copy(hr.ring, filtered)
}

// Get 根据 key 查找命中节点。若环为空返回空字符串。
func (hr *HashRing) Get(key string) string {
	hr.mu.RLock()
	defer hr.mu.RUnlock()
	if len(hr.ring) == 0 {
		return ""
	}
	hv := ringHash([]byte(key))
	idx := sort.Search(len(hr.ring), func(i int) bool { return hr.ring[i] >= hv })
	if idx == len(hr.ring) {
		idx = 0 // 环形回绕
	}
	return hr.nodesMap[hr.ring[idx]]
}

// Nodes 返回当前环中的真实节点集合（无重复）。
func (hr *HashRing) Nodes() []string {
	hr.mu.RLock()
	defer hr.mu.RUnlock()
	// 去重收集
	set := make(map[string]struct{})
	for _, n := range hr.nodesMap {
		set[n] = struct{}{}
	}
	res := make([]string, 0, len(set))
	for n := range set {
		res = append(res, n)
	}
	sort.Strings(res)
	return res
}

// Size 返回虚拟节点数量（用于观测）。
func (hr *HashRing) Size() int {
	hr.mu.RLock()
	defer hr.mu.RUnlock()
	return len(hr.ring)
}
