package main

import (
	"context"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Unstructured interface {
	ToUnstructured() (*unstructured.Unstructured, error)
	UniqueKey() string
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

func (dm *NamespacedDynamicModel) UniqueKey() string {
	return fmt.Sprintf("%s-%s-%s-%s", dm.Namespace, dm.Name, dm.ClusterID, dm.TenantID)
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

func (dm *DynamicModel) UniqueKey() string {
	return fmt.Sprintf("%s-%s-%s", dm.Name, dm.ClusterID, dm.TenantID)
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
	AutoMigrate(context.Context) error
	Find(context.Context, string, string) (*unstructured.Unstructured, error)
	Save(context.Context, *unstructured.Unstructured) error
	Create(context.Context, *unstructured.Unstructured) error
	Delete(context.Context, *unstructured.Unstructured) error
	// NeedUpdate returns true if the object needs to be updated
	// It's Useful for update new field
	// in this function you need to compare the new object and the old data in db
	// if the new object is different from the old data in db, return true
	NeedUpdate(ctx context.Context, new *unstructured.Unstructured, old any) bool
}
