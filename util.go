package main

import (
	"context"
	"fmt"
	"log"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

var DefaultHandler = Handler

func Handler(ctx context.Context, ctrl *Controller, action string, obj *unstructured.Unstructured) error {
	log.Printf("Controller %s received %s event for %s/%s\n", ctrl.name, action, obj.GetNamespace(), obj.GetName())
	//dao := NewDao()
	switch action {
	case ActionAdd:
		for _, dao := range ctrl.dao {
			err := dao.Create(ctx, obj)
			if err != nil {
				return err
			}
		}

		//// 是否已经存在
		//dbData, err := dao.Find(context.Background(), ctrl.GetGVR(), obj.GetNamespace(), obj.GetName())
		//if err != nil {
		//	return err
		//}
		//if dbData != nil {
		//	// 如果存在，进行更新
		//	return dao.Save(context.Background(), ctrl.GetGVR(), obj)
		//}
	case ActionUpdate:
	case ActionDelete:
	default:
		return fmt.Errorf("unknown action: %s", action)
	}
	return nil
}

var DefaultNeedUpdate = NeedUpdate

func NeedUpdate(old, new *unstructured.Unstructured) bool {
	return old.GetResourceVersion() != new.GetResourceVersion()
}

var DefaultUniqueFunc = Unique

func Unique(obj *unstructured.Unstructured, namespaced bool) string {
	return fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName())
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
