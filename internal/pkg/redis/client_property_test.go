package redis

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// TestProperty_SeqIDStrictIncrement tests that for any Guild's message sequence,
// Seq IDs obtained from Redis should strictly increment, ensuring message ordering
func TestProperty_SeqIDStrictIncrement(t *testing.T) {
	client := setupTestRedis(t)
	ctx := context.Background()

	properties := gopter.NewProperties(nil)

	// Property 1: Sequential calls to GenerateSeqID for the same guild produce strictly increasing IDs
	properties.Property("for any guild, sequential seq ID generation produces strictly increasing values",
		prop.ForAll(
			func(guildID string, count int) bool {
				// Generate multiple Seq IDs for the same guild
				ids := make([]int64, count)
				for i := range count {
					id, err := client.GenerateSeqID(ctx, guildID)
					if err != nil {
						t.Logf("Error generating seq ID: %v", err)
						return false
					}
					ids[i] = id
				}

				// Verify strict increment: each ID should be exactly 1 more than the previous
				for i := 1; i < len(ids); i++ {
					if ids[i] != ids[i-1]+1 {
						t.Logf("Seq IDs not strictly incrementing: %d -> %d (expected %d)", ids[i-1], ids[i], ids[i-1]+1)
						return false
					}
				}

				return true
			},
			genGuildID(),
			gen.IntRange(2, 20), // Generate 2-20 IDs per test
		))

	// Property 2: Concurrent calls to GenerateSeqID produce unique, incrementing IDs
	properties.Property("for any guild, concurrent seq ID generation produces unique incrementing values",
		prop.ForAll(
			func(guildID string, numGoroutines int, idsPerGoroutine int) bool {
				totalIDs := numGoroutines * idsPerGoroutine
				results := make(chan int64, totalIDs)
				errors := make(chan error, totalIDs)

				// Launch concurrent goroutines
				var wg sync.WaitGroup
				for range numGoroutines {
					wg.Go(func() {
						for j := 0; j < idsPerGoroutine; j++ {
							id, err := client.GenerateSeqID(ctx, guildID)
							if err != nil {
								errors <- err
								return
							}
							results <- id
						}
					})
				}
				wg.Wait()

				close(results)
				close(errors)

				// Check for errors
				if len(errors) > 0 {
					t.Logf("Errors during concurrent generation: %d", len(errors))
					return false
				}

				// Collect all IDs
				ids := make([]int64, 0, totalIDs)
				for id := range results {
					ids = append(ids, id)
				}

				// Verify all IDs are unique
				idSet := make(map[int64]bool)
				for _, id := range ids {
					if idSet[id] {
						t.Logf("Duplicate ID found: %d", id)
						return false
					}
					idSet[id] = true
				}

				// Verify IDs form a continuous sequence when sorted
				sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
				for i := 1; i < len(ids); i++ {
					if ids[i] != ids[i-1]+1 {
						t.Logf("IDs not forming continuous sequence: %d -> %d", ids[i-1], ids[i])
						return false
					}
				}

				return true
			},
			genGuildID(),
			gen.IntRange(2, 10), // 2-10 goroutines
			gen.IntRange(5, 15), // 5-15 IDs per goroutine
		))

	// Property 3: Different guilds have independent sequences
	properties.Property("for any two different guilds, their seq ID sequences are independent",
		prop.ForAll(
			func(guild1 string, guild2 string, count int) bool {
				// Ensure guilds are different
				if guild1 == guild2 {
					return true // Skip this case
				}

				// Generate IDs for both guilds in interleaved fashion
				ids1 := make([]int64, count)
				ids2 := make([]int64, count)

				for i := range count {
					id1, err := client.GenerateSeqID(ctx, guild1)
					if err != nil {
						t.Logf("Error generating seq ID for guild1: %v", err)
						return false
					}
					ids1[i] = id1

					id2, err := client.GenerateSeqID(ctx, guild2)
					if err != nil {
						t.Logf("Error generating seq ID for guild2: %v", err)
						return false
					}
					ids2[i] = id2
				}

				// Verify each guild's sequence is strictly incrementing
				for i := 1; i < count; i++ {
					if ids1[i] != ids1[i-1]+1 {
						t.Logf("Guild1 IDs not strictly incrementing: %d -> %d", ids1[i-1], ids1[i])
						return false
					}
					if ids2[i] != ids2[i-1]+1 {
						t.Logf("Guild2 IDs not strictly incrementing: %d -> %d", ids2[i-1], ids2[i])
						return false
					}
				}

				return true
			},
			genGuildID(),
			genGuildID(),
			gen.IntRange(3, 10), // Generate 3-10 IDs per guild
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty_UserOnlineStatusCache tests that for any user's online status change,
// the status should be correctly updated in Redis cache
func TestProperty_UserOnlineStatusCache(t *testing.T) {
	client := setupTestRedis(t)
	ctx := context.Background()

	properties := gopter.NewProperties(nil)

	// Property 1: Setting user online makes them queryable as online
	properties.Property("for any user, setting online status makes them queryable as online",
		prop.ForAll(
			func(userID string, ttl time.Duration) bool {
				// Set user online
				err := client.SetUserOnline(ctx, userID, ttl)
				if err != nil {
					t.Logf("Error setting user online: %v", err)
					return false
				}

				// Verify user is online
				online, err := client.IsUserOnline(ctx, userID)
				if err != nil {
					t.Logf("Error checking user online status: %v", err)
					return false
				}

				if !online {
					t.Logf("User %s should be online but is not", userID)
					return false
				}

				return true
			},
			genUserID(),
			genTTL(),
		))

	// Property 2: Removing user online status makes them queryable as offline
	properties.Property("for any user, removing online status makes them queryable as offline",
		prop.ForAll(
			func(userID string, ttl time.Duration) bool {
				// First set user online
				err := client.SetUserOnline(ctx, userID, ttl)
				if err != nil {
					t.Logf("Error setting user online: %v", err)
					return false
				}

				// Remove online status
				err = client.RemoveUserOnline(ctx, userID)
				if err != nil {
					t.Logf("Error removing user online status: %v", err)
					return false
				}

				// Verify user is offline
				online, err := client.IsUserOnline(ctx, userID)
				if err != nil {
					t.Logf("Error checking user online status: %v", err)
					return false
				}

				if online {
					t.Logf("User %s should be offline but is online", userID)
					return false
				}

				return true
			},
			genUserID(),
			genTTL(),
		))

	// Property 3: Multiple users have independent online status
	properties.Property("for any set of users, their online statuses are independent",
		prop.ForAll(
			func(users []string, ttl time.Duration) bool {
				// Ensure we have at least 2 users and they're unique
				if len(users) < 2 {
					return true // Skip
				}

				// Make users unique
				userSet := make(map[string]bool)
				uniqueUsers := make([]string, 0)
				for _, u := range users {
					if !userSet[u] {
						userSet[u] = true
						uniqueUsers = append(uniqueUsers, u)
					}
				}

				if len(uniqueUsers) < 2 {
					return true // Skip if not enough unique users
				}

				// Clean up all users first to ensure clean state
				for _, userID := range uniqueUsers {
					client.RemoveUserOnline(ctx, userID)
				}

				// Set half of the users online
				onlineUsers := make(map[string]bool)
				for i := 0; i < len(uniqueUsers)/2; i++ {
					err := client.SetUserOnline(ctx, uniqueUsers[i], ttl)
					if err != nil {
						t.Logf("Error setting user online: %v", err)
						return false
					}
					onlineUsers[uniqueUsers[i]] = true
				}

				// Verify each user's status
				for _, userID := range uniqueUsers {
					online, err := client.IsUserOnline(ctx, userID)
					if err != nil {
						t.Logf("Error checking user online status: %v", err)
						return false
					}

					expectedOnline := onlineUsers[userID]
					if online != expectedOnline {
						t.Logf("User %s online status mismatch: expected %v, got %v", userID, expectedOnline, online)
						return false
					}
				}

				// Clean up after test
				for _, userID := range uniqueUsers {
					client.RemoveUserOnline(ctx, userID)
				}

				return true
			},
			gen.SliceOfN(10, genUserID()), // Generate up to 10 users
			genTTL(),
		))

	// Property 4: Updating online status extends the TTL
	properties.Property("for any user, updating online status extends their presence",
		prop.ForAll(
			func(userID string, ttl1 time.Duration, ttl2 time.Duration) bool {
				// Set user online with first TTL
				err := client.SetUserOnline(ctx, userID, ttl1)
				if err != nil {
					t.Logf("Error setting user online: %v", err)
					return false
				}

				// Verify user is online
				online, err := client.IsUserOnline(ctx, userID)
				if err != nil {
					t.Logf("Error checking user online status: %v", err)
					return false
				}
				if !online {
					t.Logf("User should be online after first set")
					return false
				}

				// Update with second TTL
				err = client.SetUserOnline(ctx, userID, ttl2)
				if err != nil {
					t.Logf("Error updating user online status: %v", err)
					return false
				}

				// Verify user is still online
				online, err = client.IsUserOnline(ctx, userID)
				if err != nil {
					t.Logf("Error checking user online status after update: %v", err)
					return false
				}
				if !online {
					t.Logf("User should be online after update")
					return false
				}

				return true
			},
			genUserID(),
			genTTL(),
			genTTL(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
