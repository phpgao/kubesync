package main

import (
	"context"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Unstructured interface {
	ToUnstructured() (*unstructured.Unstructured, error)
	TableName() string
}

type JSONMap map[string]interface{}

// NamespacedDynamicModel 基础模型，包含通用字段
type NamespacedDynamicModel struct {
	Namespace string
	DynamicModel
}

// TableName 动态生成表名，格式: gvk_group_version_kind
func (dm *NamespacedDynamicModel) TableName() string {
	return dm.Resource
}

// DynamicModel 基础模型，包含通用字段
type DynamicModel struct {
	gorm.Model
	Name      string
	Version   string
	Resource  string `gorm:"-"`
	Raw       string
	ClusterID string
	TenantID  string
}

// TableName 动态生成表名，格式: gvk_group_version_kind
func (dm *DynamicModel) TableName() string {
	return dm.Resource
}

func (dm *DynamicModel) ToUnstructured() (*unstructured.Unstructured, error) {
	if dm.Raw == "" {
		utd := &unstructured.Unstructured{}
		err := json.Unmarshal([]byte(dm.Raw), &utd.Object)
		if err != nil {
			return nil, err
		}
		return utd, nil
	}

	return nil, fmt.Errorf("invalid raw: %s", dm.Raw)
}

type ResourceStorage interface {
	AutoMigrate(context.Context, schema.GroupVersionResource, bool) error
	Find(context.Context, schema.GroupVersionResource, bool, string, string) (*unstructured.Unstructured, error)
	Save(context.Context, schema.GroupVersionResource, bool, *unstructured.Unstructured) error
	Create(context.Context, schema.GroupVersionResource, bool, *unstructured.Unstructured) error
	Delete(context.Context, schema.GroupVersionResource, bool, *unstructured.Unstructured) error
	// NeedUpdate returns true if the object needs to be updated
	// It's Useful for update new field
	NeedUpdate(ctx context.Context, gvr schema.GroupVersionResource, namespaced bool, new *unstructured.Unstructured, old *unstructured.Unstructured) bool
}
