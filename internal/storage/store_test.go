package storage

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/freewebtopdf/asset-injector/internal/domain"

	"github.com/google/uuid"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_BasicOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "store_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	store := NewStore(tempDir)
	ctx := context.Background()

	err = store.Load(ctx)
	require.NoError(t, err)

	rule := &domain.Rule{
		ID:      uuid.New().String(),
		Type:    "exact",
		Pattern: "https://example.com",
		CSS:     "body { display: none; }",
		JS:      "console.log('test');",
	}

	err = store.CreateRule(ctx, rule)
	require.NoError(t, err)

	retrieved, err := store.GetRuleByID(ctx, rule.ID)
	require.NoError(t, err)
	assert.Equal(t, rule.ID, retrieved.ID)
	assert.Equal(t, rule.Type, retrieved.Type)
	assert.Equal(t, rule.Pattern, retrieved.Pattern)
	assert.Equal(t, rule.CSS, retrieved.CSS)
	assert.Equal(t, rule.JS, retrieved.JS)

	allRules, err := store.GetAllRules(ctx)
	require.NoError(t, err)
	assert.Len(t, allRules, 1)
	assert.Equal(t, rule.ID, allRules[0].ID)

	rule.CSS = "body { background: red; }"
	originalCreatedAt := retrieved.CreatedAt
	time.Sleep(time.Millisecond)

	err = store.UpdateRule(ctx, rule)
	require.NoError(t, err)

	updated, err := store.GetRuleByID(ctx, rule.ID)
	require.NoError(t, err)
	assert.Equal(t, rule.CSS, updated.CSS)
	assert.Equal(t, originalCreatedAt, updated.CreatedAt)
	assert.True(t, updated.UpdatedAt.After(originalCreatedAt))

	err = store.DeleteRule(ctx, rule.ID)
	require.NoError(t, err)

	_, err = store.GetRuleByID(ctx, rule.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NOT_FOUND")

	allRules, err = store.GetAllRules(ctx)
	require.NoError(t, err)
	assert.Len(t, allRules, 0)
}

func TestStore_Persistence(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "store_persistence_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	store1 := NewStore(tempDir)
	ctx := context.Background()

	err = store1.Load(ctx)
	require.NoError(t, err)

	rule1 := &domain.Rule{
		ID:      uuid.New().String(),
		Type:    "exact",
		Pattern: "https://example.com",
		CSS:     "body { display: none; }",
	}

	rule2 := &domain.Rule{
		ID:      uuid.New().String(),
		Type:    "regex",
		Pattern: "https://.*\\.example\\.com",
		JS:      "console.log('test');",
	}

	err = store1.CreateRule(ctx, rule1)
	require.NoError(t, err)

	err = store1.CreateRule(ctx, rule2)
	require.NoError(t, err)

	store2 := NewStore(tempDir)
	err = store2.Load(ctx)
	require.NoError(t, err)

	allRules, err := store2.GetAllRules(ctx)
	require.NoError(t, err)
	assert.Len(t, allRules, 2)

	retrieved1, err := store2.GetRuleByID(ctx, rule1.ID)
	require.NoError(t, err)
	assert.Equal(t, rule1.Pattern, retrieved1.Pattern)
	assert.Equal(t, rule1.CSS, retrieved1.CSS)

	retrieved2, err := store2.GetRuleByID(ctx, rule2.ID)
	require.NoError(t, err)
	assert.Equal(t, rule2.Pattern, retrieved2.Pattern)
	assert.Equal(t, rule2.JS, retrieved2.JS)
}

func genRule() gopter.Gen {
	return gopter.CombineGens(
		gen.OneConstOf("exact", "regex", "wildcard"),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) <= 100 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) <= 1000 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) <= 1000 }),
	).Map(func(values []interface{}) *domain.Rule {
		return &domain.Rule{
			ID:      uuid.New().String(),
			Type:    values[0].(string),
			Pattern: values[1].(string),
			CSS:     values[2].(string),
			JS:      values[3].(string),
		}
	})
}

func genUniqueRules(maxSize int) gopter.Gen {
	return gen.IntRange(0, maxSize).FlatMap(func(sizeInterface interface{}) gopter.Gen {
		size := sizeInterface.(int)
		return gen.SliceOfN(size, genRule())
	}, reflect.TypeOf([]*domain.Rule{}))
}

func TestProperty_DualIndexSynchronization(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("map and slice should contain identical rule data after operations", prop.ForAll(
		func(rules []*domain.Rule) bool {
			tempDir, err := os.MkdirTemp("", "dual_index_test")
			if err != nil {
				return false
			}
			defer os.RemoveAll(tempDir)

			store := NewStore(tempDir)
			ctx := context.Background()

			if err := store.Load(ctx); err != nil {
				return false
			}

			for _, rule := range rules {
				if err := store.CreateRule(ctx, rule); err != nil {
					return false
				}
			}

			allRules, err := store.GetAllRules(ctx)
			if err != nil {
				return false
			}

			if len(allRules) != len(rules) {
				return false
			}

			for _, rule := range allRules {
				retrieved, err := store.GetRuleByID(ctx, rule.ID)
				if err != nil {
					return false
				}
				if retrieved.ID != rule.ID || retrieved.Pattern != rule.Pattern {
					return false
				}
			}

			return true
		},
		genUniqueRules(10),
	))

	properties.TestingRun(t)
}

func TestProperty_CompleteRuleRetrieval(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("GetAllRules should return every rule that has been created and not deleted", prop.ForAll(
		func(rules []*domain.Rule) bool {
			tempDir, err := os.MkdirTemp("", "complete_retrieval_test")
			if err != nil {
				return false
			}
			defer os.RemoveAll(tempDir)

			store := NewStore(tempDir)
			ctx := context.Background()

			if err := store.Load(ctx); err != nil {
				return false
			}

			for _, rule := range rules {
				if err := store.CreateRule(ctx, rule); err != nil {
					return false
				}
			}

			allRules, err := store.GetAllRules(ctx)
			if err != nil {
				return false
			}

			if len(allRules) != len(rules) {
				return false
			}

			ruleMap := make(map[string]domain.Rule)
			for _, rule := range allRules {
				ruleMap[rule.ID] = rule
			}

			for _, originalRule := range rules {
				if retrievedRule, exists := ruleMap[originalRule.ID]; !exists {
					return false
				} else if retrievedRule.Pattern != originalRule.Pattern {
					return false
				}
			}

			return true
		},
		genUniqueRules(10),
	))

	properties.TestingRun(t)
}

func TestProperty_RuleDeletionCompleteness(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("deleting a rule by ID should remove it from both memory storage and subsequent queries", prop.ForAll(
		func(rules []*domain.Rule, deleteIndex int) bool {
			if len(rules) == 0 {
				return true
			}

			tempDir, err := os.MkdirTemp("", "deletion_test")
			if err != nil {
				return false
			}
			defer os.RemoveAll(tempDir)

			store := NewStore(tempDir)
			ctx := context.Background()

			if err := store.Load(ctx); err != nil {
				return false
			}

			for _, rule := range rules {
				if err := store.CreateRule(ctx, rule); err != nil {
					return false
				}
			}

			deleteIndex = deleteIndex % len(rules)
			ruleToDelete := rules[deleteIndex]

			if err := store.DeleteRule(ctx, ruleToDelete.ID); err != nil {
				return false
			}

			_, err = store.GetRuleByID(ctx, ruleToDelete.ID)
			if err == nil {
				return false
			}

			allRules, err := store.GetAllRules(ctx)
			if err != nil {
				return false
			}

			for _, rule := range allRules {
				if rule.ID == ruleToDelete.ID {
					return false
				}
			}

			return len(allRules) == len(rules)-1
		},
		genUniqueRules(10).SuchThat(func(rules []*domain.Rule) bool {
			return len(rules) > 0
		}),
		gen.IntRange(0, 100),
	))

	properties.TestingRun(t)
}

func TestProperty_TimestampPreservationOnUpdate(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("updating a rule should preserve CreatedAt timestamp while updating UpdatedAt", prop.ForAll(
		func(rule *domain.Rule, newCSS string) bool {
			tempDir, err := os.MkdirTemp("", "timestamp_test")
			if err != nil {
				return false
			}
			defer os.RemoveAll(tempDir)

			store := NewStore(tempDir)
			ctx := context.Background()

			if err := store.Load(ctx); err != nil {
				return false
			}

			if err := store.CreateRule(ctx, rule); err != nil {
				return false
			}

			created, err := store.GetRuleByID(ctx, rule.ID)
			if err != nil {
				return false
			}

			originalCreatedAt := created.CreatedAt
			originalUpdatedAt := created.UpdatedAt

			time.Sleep(time.Millisecond)

			rule.CSS = newCSS
			if err := store.UpdateRule(ctx, rule); err != nil {
				return false
			}

			updated, err := store.GetRuleByID(ctx, rule.ID)
			if err != nil {
				return false
			}

			return updated.CreatedAt.Equal(originalCreatedAt) && updated.UpdatedAt.After(originalUpdatedAt)
		},
		genRule(),
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}

func TestProperty_ThreadSafeConcurrentAccess(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("concurrent read and write operations should maintain data integrity", prop.ForAll(
		func(rules []*domain.Rule) bool {
			if len(rules) == 0 {
				return true
			}

			tempDir, err := os.MkdirTemp("", "concurrent_test")
			if err != nil {
				return false
			}
			defer os.RemoveAll(tempDir)

			store := NewStore(tempDir)
			ctx := context.Background()

			if err := store.Load(ctx); err != nil {
				return false
			}

			for _, rule := range rules {
				if err := store.CreateRule(ctx, rule); err != nil {
					return false
				}
			}

			done := make(chan bool, 10)

			for i := 0; i < 5; i++ {
				go func() {
					defer func() { done <- true }()
					for j := 0; j < 10; j++ {
						_, _ = store.GetAllRules(ctx)
					}
				}()
			}

			for i := 0; i < 2; i++ {
				go func(index int) {
					defer func() { done <- true }()
					if index < len(rules) {
						rule := rules[index]
						rule.CSS = "updated"
						_ = store.UpdateRule(ctx, rule)
					}
				}(i)
			}

			for i := 0; i < 7; i++ {
				<-done
			}

			allRules, err := store.GetAllRules(ctx)
			if err != nil {
				return false
			}

			return len(allRules) == len(rules)
		},
		genUniqueRules(5).SuchThat(func(rules []*domain.Rule) bool {
			return len(rules) > 0
		}),
	))

	properties.TestingRun(t)
}
