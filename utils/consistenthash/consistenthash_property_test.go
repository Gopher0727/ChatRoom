package consistenthash

import (
	"fmt"
	"testing"

	"pgregory.net/rapid"
)

// Property: For any new Gateway node startup, it should be correctly added to the consistent hash ring;
// For any node going offline, it should be removed from the hash ring
func TestProperty_ConsistentHashNodeManagement(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping property test in short mode")
	}

	rapid.Check(t, func(rt *rapid.T) {
		// Create a new consistent hash ring with random replicas
		replicas := rapid.IntRange(10, 200).Draw(rt, "replicas")
		ring := New(replicas, nil)

		// Generate a set of initial nodes
		numInitialNodes := rapid.IntRange(1, 10).Draw(rt, "numInitialNodes")
		initialNodes := make([]string, numInitialNodes)
		for i := 0; i < numInitialNodes; i++ {
			initialNodes[i] = fmt.Sprintf("node_%d", i)
		}

		// Add initial nodes
		ring.Add(initialNodes...)

		// Property 1: All added nodes should be in the ring
		if ring.Size() != numInitialNodes {
			rt.Fatalf("Expected %d nodes in ring, got %d", numInitialNodes, ring.Size())
		}

		// Property 2: Each node should have exactly 'replicas' virtual nodes
		expectedVirtualNodes := numInitialNodes * replicas
		if len(ring.keys) != expectedVirtualNodes {
			rt.Fatalf("Expected %d virtual nodes, got %d", expectedVirtualNodes, len(ring.keys))
		}

		// Property 3: All initial nodes should be retrievable
		nodes := ring.Nodes()
		nodeMap := make(map[string]bool)
		for _, node := range nodes {
			nodeMap[node] = true
		}
		for _, expectedNode := range initialNodes {
			if !nodeMap[expectedNode] {
				rt.Fatalf("Node %s should be in the ring", expectedNode)
			}
		}

		// Property 4: Keys should be sorted
		for i := 1; i < len(ring.keys); i++ {
			if ring.keys[i-1] >= ring.keys[i] {
				rt.Fatalf("Keys are not sorted at index %d", i)
			}
		}

		// Test adding a new node (simulating Gateway startup)
		newNode := fmt.Sprintf("node_%d", numInitialNodes)
		ring.Add(newNode)

		// Property 5: New node should be added to the ring
		if ring.Size() != numInitialNodes+1 {
			rt.Fatalf("Expected %d nodes after adding new node, got %d", numInitialNodes+1, ring.Size())
		}

		// Property 6: Virtual nodes should increase by replicas
		expectedVirtualNodes = (numInitialNodes + 1) * replicas
		if len(ring.keys) != expectedVirtualNodes {
			rt.Fatalf("Expected %d virtual nodes after adding node, got %d", expectedVirtualNodes, len(ring.keys))
		}

		// Property 7: Keys should still be sorted after adding
		for i := 1; i < len(ring.keys); i++ {
			if ring.keys[i-1] >= ring.keys[i] {
				rt.Fatalf("Keys are not sorted after adding node at index %d", i)
			}
		}

		// Property 8: New node should be retrievable
		nodes = ring.Nodes()
		found := false
		for _, node := range nodes {
			if node == newNode {
				found = true
				break
			}
		}
		if !found {
			rt.Fatalf("New node %s should be in the ring", newNode)
		}

		// Test removing a node (simulating Gateway going offline)
		if numInitialNodes > 0 {
			nodeToRemove := initialNodes[0]
			ring.Remove(nodeToRemove)

			// Property 9: Removed node should not be in the ring
			if ring.Size() != numInitialNodes {
				rt.Fatalf("Expected %d nodes after removing node, got %d", numInitialNodes, ring.Size())
			}

			// Property 10: Virtual nodes should decrease by replicas
			expectedVirtualNodes = numInitialNodes * replicas
			if len(ring.keys) != expectedVirtualNodes {
				rt.Fatalf("Expected %d virtual nodes after removing node, got %d", expectedVirtualNodes, len(ring.keys))
			}

			// Property 11: Keys should still be sorted after removing
			for i := 1; i < len(ring.keys); i++ {
				if ring.keys[i-1] >= ring.keys[i] {
					rt.Fatalf("Keys are not sorted after removing node at index %d", i)
				}
			}

			// Property 12: Removed node should not be retrievable
			nodes = ring.Nodes()
			for _, node := range nodes {
				if node == nodeToRemove {
					rt.Fatalf("Removed node %s should not be in the ring", nodeToRemove)
				}
			}
		}

		// Property 13: Ring should still function correctly after add/remove
		// Test that we can still get nodes for keys
		testKey := rapid.String().Draw(rt, "testKey")
		node := ring.Get(testKey)
		if ring.Size() > 0 && node == "" {
			rt.Fatalf("Should be able to get a node for key when ring is not empty")
		}
		if ring.Size() == 0 && node != "" {
			rt.Fatalf("Should return empty string when ring is empty")
		}
	})
}

// Property: When adding or removing nodes, only a minimal portion of keys should be remapped
func TestProperty_ConsistentHashMinimalDisruption(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping property test in short mode")
	}

	rapid.Check(t, func(rt *rapid.T) {
		// Create a ring with good number of replicas for better distribution
		replicas := rapid.IntRange(50, 150).Draw(rt, "replicas")
		ring := New(replicas, nil)

		// Add initial nodes (at least 3 for meaningful test)
		numInitialNodes := rapid.IntRange(3, 8).Draw(rt, "numInitialNodes")
		initialNodes := make([]string, numInitialNodes)
		for i := 0; i < numInitialNodes; i++ {
			initialNodes[i] = fmt.Sprintf("node_%d", i)
		}
		ring.Add(initialNodes...)

		// Generate test keys
		numKeys := rapid.IntRange(100, 500).Draw(rt, "numKeys")
		testKeys := make([]string, numKeys)
		for i := 0; i < numKeys; i++ {
			testKeys[i] = fmt.Sprintf("key_%d", i)
		}

		// Record initial mapping
		initialMapping := make(map[string]string)
		for _, key := range testKeys {
			initialMapping[key] = ring.Get(key)
		}

		// Add a new node
		newNode := fmt.Sprintf("node_%d", numInitialNodes)
		ring.Add(newNode)

		// Count how many keys changed
		changedAfterAdd := 0
		for _, key := range testKeys {
			if ring.Get(key) != initialMapping[key] {
				changedAfterAdd++
			}
		}

		// Property: When adding a node, at most approximately 1/(n+1) of keys should change
		// We use a generous tolerance since hash distribution is probabilistic
		maxExpectedChange := float64(numKeys) / float64(numInitialNodes+1)
		tolerance := maxExpectedChange * 2.0 // Allow 2x tolerance for randomness

		if float64(changedAfterAdd) > tolerance {
			rt.Fatalf("Too many keys changed after adding node: %d/%d (expected < %.0f)",
				changedAfterAdd, numKeys, tolerance)
		}

		// Record mapping after add
		afterAddMapping := make(map[string]string)
		for _, key := range testKeys {
			afterAddMapping[key] = ring.Get(key)
		}

		// Remove the new node
		ring.Remove(newNode)

		// Count how many keys changed back
		changedAfterRemove := 0
		for _, key := range testKeys {
			if ring.Get(key) != afterAddMapping[key] {
				changedAfterRemove++
			}
		}

		// Property: When removing a node, at most approximately 1/n of keys should change
		maxExpectedChange = float64(numKeys) / float64(numInitialNodes+1)
		tolerance = maxExpectedChange * 2.0

		if float64(changedAfterRemove) > tolerance {
			rt.Fatalf("Too many keys changed after removing node: %d/%d (expected < %.0f)",
				changedAfterRemove, numKeys, tolerance)
		}

		// Property: Keys that didn't map to the removed node should not change
		unchangedCount := 0
		for _, key := range testKeys {
			if afterAddMapping[key] != newNode && ring.Get(key) == initialMapping[key] {
				unchangedCount++
			}
		}

		// Most keys that didn't map to the removed node should remain unchanged
		// (some might change due to hash collisions, but should be minimal)
		keysNotOnRemovedNode := 0
		for _, key := range testKeys {
			if afterAddMapping[key] != newNode {
				keysNotOnRemovedNode++
			}
		}

		if keysNotOnRemovedNode > 0 {
			unchangedRatio := float64(unchangedCount) / float64(keysNotOnRemovedNode)
			if unchangedRatio < 0.5 { // At least 50% should remain unchanged
				rt.Fatalf("Too many keys changed that weren't on removed node: %.2f%% unchanged",
					unchangedRatio*100)
			}
		}
	})
}

// Property: Keys should be distributed relatively evenly across all nodes
func TestProperty_ConsistentHashLoadBalance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping property test in short mode")
	}

	rapid.Check(t, func(rt *rapid.T) {
		// Create a ring with good number of replicas for better distribution
		replicas := rapid.IntRange(100, 200).Draw(rt, "replicas")
		ring := New(replicas, nil)

		// Add nodes (at least 3 for meaningful distribution test)
		numNodes := rapid.IntRange(3, 10).Draw(rt, "numNodes")
		nodes := make([]string, numNodes)
		for i := 0; i < numNodes; i++ {
			nodes[i] = fmt.Sprintf("node_%d", i)
		}
		ring.Add(nodes...)

		// Generate a large number of keys
		numKeys := rapid.IntRange(500, 2000).Draw(rt, "numKeys")
		distribution := make(map[string]int)

		for i := 0; i < numKeys; i++ {
			key := fmt.Sprintf("key_%d", i)
			node := ring.Get(key)
			distribution[node]++
		}

		// Property: All nodes should be used
		if len(distribution) != numNodes {
			rt.Fatalf("Expected all %d nodes to be used, but only %d were used",
				numNodes, len(distribution))
		}

		// Property: Distribution should be relatively even
		// Each node should get approximately numKeys/numNodes keys
		expectedPerNode := float64(numKeys) / float64(numNodes)
		tolerance := expectedPerNode * 0.5 // 50% tolerance for randomness

		for node, count := range distribution {
			diff := float64(count) - expectedPerNode
			if diff < 0 {
				diff = -diff
			}
			if diff > tolerance {
				rt.Fatalf("Node %s has uneven distribution: %d keys (expected %.0f ± %.0f)",
					node, count, expectedPerNode, tolerance)
			}
		}

		// Property: Standard deviation should be reasonable
		// Calculate mean
		mean := float64(numKeys) / float64(numNodes)

		// Calculate variance
		variance := 0.0
		for _, count := range distribution {
			diff := float64(count) - mean
			variance += diff * diff
		}
		variance /= float64(numNodes)

		// Standard deviation should be less than 30% of mean
		stdDev := 0.0
		if variance > 0 {
			stdDev = 1.0
			for i := 0; i < 10; i++ { // Simple sqrt approximation
				stdDev = (stdDev + variance/stdDev) / 2
			}
		}

		maxStdDev := mean * 0.3
		if stdDev > maxStdDev {
			rt.Fatalf("Standard deviation too high: %.2f (expected < %.2f, mean=%.2f)",
				stdDev, maxStdDev, mean)
		}
	})
}

// Property: The same key should always map to the same node (consistency)
func TestProperty_ConsistentHashConsistency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping property test in short mode")
	}

	rapid.Check(t, func(rt *rapid.T) {
		// Create a ring
		replicas := rapid.IntRange(10, 100).Draw(rt, "replicas")
		ring := New(replicas, nil)

		// Add nodes
		numNodes := rapid.IntRange(1, 10).Draw(rt, "numNodes")
		nodes := make([]string, numNodes)
		for i := 0; i < numNodes; i++ {
			nodes[i] = fmt.Sprintf("node_%d", i)
		}
		ring.Add(nodes...)

		// Generate test keys
		numKeys := rapid.IntRange(10, 100).Draw(rt, "numKeys")
		testKeys := make([]string, numKeys)
		for i := 0; i < numKeys; i++ {
			testKeys[i] = rapid.String().Draw(rt, fmt.Sprintf("key_%d", i))
		}

		// Property: Same key should always return same node
		for _, key := range testKeys {
			node1 := ring.Get(key)
			node2 := ring.Get(key)
			node3 := ring.Get(key)

			if node1 != node2 || node2 != node3 {
				rt.Fatalf("Key %s mapped to different nodes: %s, %s, %s",
					key, node1, node2, node3)
			}
		}

		// Property: GetN should also be consistent
		for _, key := range testKeys {
			n := rapid.IntRange(1, numNodes).Draw(rt, "n")
			nodes1 := ring.GetN(key, n)
			nodes2 := ring.GetN(key, n)

			if len(nodes1) != len(nodes2) {
				rt.Fatalf("GetN returned different number of nodes for key %s", key)
			}

			for i := range nodes1 {
				if nodes1[i] != nodes2[i] {
					rt.Fatalf("GetN returned different nodes for key %s at position %d", key, i)
				}
			}
		}
	})
}
