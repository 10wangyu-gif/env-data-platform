package gateway

import (
	"crypto/md5"
	"fmt"
	"hash/crc32"
	"math/rand"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// LoadBalanceStrategy 负载均衡策略
type LoadBalanceStrategy string

const (
	RoundRobin       LoadBalanceStrategy = "round_robin"
	WeightedRoundRobin LoadBalanceStrategy = "weighted_round_robin"
	LeastConnections LoadBalanceStrategy = "least_connections"
	ConsistentHash   LoadBalanceStrategy = "consistent_hash"
	Random           LoadBalanceStrategy = "random"
)

// Target 目标服务器
type Target struct {
	ID          string            `json:"id" yaml:"id"`
	URL         string            `json:"url" yaml:"url"`
	Weight      int               `json:"weight" yaml:"weight"`
	Metadata    map[string]string `json:"metadata" yaml:"metadata"`
	Connections int               `json:"connections"`
	IsHealthy   bool              `json:"is_healthy"`
	LastCheck   time.Time         `json:"last_check"`
}

// ServiceGroup 服务组
type ServiceGroup struct {
	ID       string               `json:"id" yaml:"id"`
	Strategy LoadBalanceStrategy  `json:"strategy" yaml:"strategy"`
	Targets  []*Target            `json:"targets" yaml:"targets"`
	HashKey  string               `json:"hash_key" yaml:"hash_key"`
	mutex    sync.RWMutex
	current  int
}

// LoadBalancer 负载均衡器
type LoadBalancer struct {
	groups   map[string]*ServiceGroup
	mutex    sync.RWMutex
	logger   *zap.Logger
	hashRing *ConsistentHashRing
}

// ConsistentHashRing 一致性哈希环
type ConsistentHashRing struct {
	ring     map[uint32]string
	sortedKeys []uint32
	targets    map[string]*Target
	virtualNodes int
	mutex      sync.RWMutex
}

// NewLoadBalancer 创建负载均衡器
func NewLoadBalancer(logger *zap.Logger) *LoadBalancer {
	return &LoadBalancer{
		groups:   make(map[string]*ServiceGroup),
		logger:   logger,
		hashRing: NewConsistentHashRing(100),
	}
}

// NewConsistentHashRing 创建一致性哈希环
func NewConsistentHashRing(virtualNodes int) *ConsistentHashRing {
	return &ConsistentHashRing{
		ring:         make(map[uint32]string),
		targets:      make(map[string]*Target),
		virtualNodes: virtualNodes,
	}
}

// AddServiceGroup 添加服务组
func (lb *LoadBalancer) AddServiceGroup(group *ServiceGroup) {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()

	lb.groups[group.ID] = group

	// 如果使用一致性哈希，更新哈希环
	if group.Strategy == ConsistentHash {
		for _, target := range group.Targets {
			lb.hashRing.AddTarget(target)
		}
	}

	lb.logger.Info("Service group added",
		zap.String("group_id", group.ID),
		zap.String("strategy", string(group.Strategy)),
		zap.Int("targets", len(group.Targets)))
}

// RemoveServiceGroup 删除服务组
func (lb *LoadBalancer) RemoveServiceGroup(groupID string) {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()

	if group, exists := lb.groups[groupID]; exists {
		// 从哈希环中移除目标
		if group.Strategy == ConsistentHash {
			for _, target := range group.Targets {
				lb.hashRing.RemoveTarget(target.ID)
			}
		}
		delete(lb.groups, groupID)

		lb.logger.Info("Service group removed",
			zap.String("group_id", groupID))
	}
}

// AddTarget 向服务组添加目标
func (lb *LoadBalancer) AddTarget(groupID string, target *Target) error {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()

	group, exists := lb.groups[groupID]
	if !exists {
		return fmt.Errorf("service group %s not found", groupID)
	}

	group.mutex.Lock()
	defer group.mutex.Unlock()

	group.Targets = append(group.Targets, target)

	// 更新哈希环
	if group.Strategy == ConsistentHash {
		lb.hashRing.AddTarget(target)
	}

	lb.logger.Info("Target added to group",
		zap.String("group_id", groupID),
		zap.String("target_id", target.ID),
		zap.String("target_url", target.URL))

	return nil
}

// RemoveTarget 从服务组移除目标
func (lb *LoadBalancer) RemoveTarget(groupID, targetID string) error {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()

	group, exists := lb.groups[groupID]
	if !exists {
		return fmt.Errorf("service group %s not found", groupID)
	}

	group.mutex.Lock()
	defer group.mutex.Unlock()

	for i, target := range group.Targets {
		if target.ID == targetID {
			// 从切片中移除
			group.Targets = append(group.Targets[:i], group.Targets[i+1:]...)

			// 从哈希环中移除
			if group.Strategy == ConsistentHash {
				lb.hashRing.RemoveTarget(targetID)
			}

			lb.logger.Info("Target removed from group",
				zap.String("group_id", groupID),
				zap.String("target_id", targetID))
			return nil
		}
	}

	return fmt.Errorf("target %s not found in group %s", targetID, groupID)
}

// SelectTarget 选择目标服务器
func (lb *LoadBalancer) SelectTarget(groupID string) string {
	lb.mutex.RLock()
	group, exists := lb.groups[groupID]
	lb.mutex.RUnlock()

	if !exists {
		lb.logger.Warn("Service group not found", zap.String("group_id", groupID))
		return ""
	}

	group.mutex.Lock()
	defer group.mutex.Unlock()

	// 过滤健康的目标
	healthyTargets := make([]*Target, 0)
	for _, target := range group.Targets {
		if target.IsHealthy {
			healthyTargets = append(healthyTargets, target)
		}
	}

	if len(healthyTargets) == 0 {
		lb.logger.Warn("No healthy targets available", zap.String("group_id", groupID))
		return ""
	}

	var selected *Target

	switch group.Strategy {
	case RoundRobin:
		selected = lb.roundRobin(group, healthyTargets)
	case WeightedRoundRobin:
		selected = lb.weightedRoundRobin(group, healthyTargets)
	case LeastConnections:
		selected = lb.leastConnections(healthyTargets)
	case ConsistentHash:
		if group.HashKey != "" {
			return lb.hashRing.GetTarget(group.HashKey)
		}
		selected = lb.roundRobin(group, healthyTargets)
	case Random:
		selected = lb.random(healthyTargets)
	default:
		selected = lb.roundRobin(group, healthyTargets)
	}

	if selected != nil {
		selected.Connections++
		return selected.URL
	}

	return ""
}

// roundRobin 轮询策略
func (lb *LoadBalancer) roundRobin(group *ServiceGroup, targets []*Target) *Target {
	if len(targets) == 0 {
		return nil
	}

	target := targets[group.current%len(targets)]
	group.current++
	return target
}

// weightedRoundRobin 加权轮询策略
func (lb *LoadBalancer) weightedRoundRobin(group *ServiceGroup, targets []*Target) *Target {
	if len(targets) == 0 {
		return nil
	}

	// 计算总权重
	totalWeight := 0
	for _, target := range targets {
		totalWeight += target.Weight
	}

	if totalWeight == 0 {
		return lb.roundRobin(group, targets)
	}

	// 根据权重选择
	random := rand.Intn(totalWeight)
	current := 0
	for _, target := range targets {
		current += target.Weight
		if random < current {
			return target
		}
	}

	return targets[0]
}

// leastConnections 最少连接策略
func (lb *LoadBalancer) leastConnections(targets []*Target) *Target {
	if len(targets) == 0 {
		return nil
	}

	var selected *Target
	minConnections := int(^uint(0) >> 1) // max int

	for _, target := range targets {
		if target.Connections < minConnections {
			minConnections = target.Connections
			selected = target
		}
	}

	return selected
}

// random 随机策略
func (lb *LoadBalancer) random(targets []*Target) *Target {
	if len(targets) == 0 {
		return nil
	}

	return targets[rand.Intn(len(targets))]
}

// AddTarget 向哈希环添加目标
func (chr *ConsistentHashRing) AddTarget(target *Target) {
	chr.mutex.Lock()
	defer chr.mutex.Unlock()

	chr.targets[target.ID] = target

	// 添加虚拟节点
	for i := 0; i < chr.virtualNodes; i++ {
		hash := chr.hash(fmt.Sprintf("%s:%d", target.ID, i))
		chr.ring[hash] = target.ID
		chr.sortedKeys = append(chr.sortedKeys, hash)
	}

	sort.Slice(chr.sortedKeys, func(i, j int) bool {
		return chr.sortedKeys[i] < chr.sortedKeys[j]
	})
}

// RemoveTarget 从哈希环移除目标
func (chr *ConsistentHashRing) RemoveTarget(targetID string) {
	chr.mutex.Lock()
	defer chr.mutex.Unlock()

	delete(chr.targets, targetID)

	// 移除虚拟节点
	newKeys := make([]uint32, 0)
	for _, key := range chr.sortedKeys {
		if chr.ring[key] != targetID {
			newKeys = append(newKeys, key)
		} else {
			delete(chr.ring, key)
		}
	}
	chr.sortedKeys = newKeys
}

// GetTarget 根据键获取目标
func (chr *ConsistentHashRing) GetTarget(key string) string {
	chr.mutex.RLock()
	defer chr.mutex.RUnlock()

	if len(chr.sortedKeys) == 0 {
		return ""
	}

	hash := chr.hash(key)

	// 找到第一个大于等于hash的节点
	idx := sort.Search(len(chr.sortedKeys), func(i int) bool {
		return chr.sortedKeys[i] >= hash
	})

	// 如果没找到，使用第一个节点
	if idx == len(chr.sortedKeys) {
		idx = 0
	}

	targetID := chr.ring[chr.sortedKeys[idx]]
	if target, exists := chr.targets[targetID]; exists {
		return target.URL
	}

	return ""
}

// hash 计算哈希值
func (chr *ConsistentHashRing) hash(key string) uint32 {
	return crc32.ChecksumIEEE([]byte(key))
}

// UpdateTargetHealth 更新目标健康状态
func (lb *LoadBalancer) UpdateTargetHealth(groupID, targetID string, isHealthy bool) {
	lb.mutex.RLock()
	group, exists := lb.groups[groupID]
	lb.mutex.RUnlock()

	if !exists {
		return
	}

	group.mutex.Lock()
	defer group.mutex.Unlock()

	for _, target := range group.Targets {
		if target.ID == targetID {
			target.IsHealthy = isHealthy
			target.LastCheck = time.Now()

			lb.logger.Info("Target health updated",
				zap.String("group_id", groupID),
				zap.String("target_id", targetID),
				zap.Bool("is_healthy", isHealthy))
			return
		}
	}
}

// GetStats 获取负载均衡统计信息
func (lb *LoadBalancer) GetStats() map[string]interface{} {
	lb.mutex.RLock()
	defer lb.mutex.RUnlock()

	stats := make(map[string]interface{})

	for groupID, group := range lb.groups {
		group.mutex.RLock()
		groupStats := map[string]interface{}{
			"strategy":      string(group.Strategy),
			"total_targets": len(group.Targets),
			"healthy_targets": func() int {
				count := 0
				for _, target := range group.Targets {
					if target.IsHealthy {
						count++
					}
				}
				return count
			}(),
			"targets": group.Targets,
		}
		group.mutex.RUnlock()

		stats[groupID] = groupStats
	}

	return stats
}