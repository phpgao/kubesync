package main

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// HasCycle 深度优先循环检测函数
func HasCycle(depMap map[schema.GroupVersionResource][]schema.GroupVersionResource) bool {
	visited := make(map[schema.GroupVersionResource]bool)
	recursionStack := make(map[schema.GroupVersionResource]bool)

	var detectCycle func(node schema.GroupVersionResource) bool
	detectCycle = func(node schema.GroupVersionResource) bool {
		if recursionStack[node] {
			return true
		}
		if visited[node] {
			return false
		}

		visited[node] = true
		recursionStack[node] = true

		for _, neighbor := range depMap[node] {
			if detectCycle(neighbor) {
				return true
			}
		}

		recursionStack[node] = false
		return false
	}

	for node := range depMap {
		if !visited[node] {
			if detectCycle(node) {
				return true
			}
		}
	}
	return false
}

// 添加辅助函数
func stringSliceContains(slice []string, target string) bool {
	for _, item := range slice {
		if item == target {
			return true
		}
	}
	return false
}
