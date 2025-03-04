package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"gorm.io/gorm"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type BaseModel interface {
	ToUnstructured() (*unstructured.Unstructured, error)
	UniqueKey() string
	TableName() string
}

// DynamicModel 基础模型，包含通用字段
type DynamicModel struct {
	gorm.Model
	Name            string `gorm:"column:Name;size:255"`
	NameSpace       string `gorm:"column:Namespace;size:255"`
	Version         string `gorm:"column:Version"`
	Resource        string `gorm:"-"`
	UID             string `gorm:"column:UID;size:255;uniqueIndex:idx_uid"`
	ResourceVersion string `gorm:"column:ResourceVersion"`
	Labels          string `gorm:"column:Labels;type:text"`
	Annotations     string `gorm:"column:Annotations;type:text"`
	Raw             string `gorm:"column:Raw;type:text"`
	ClusterID       string `gorm:"column:ClusterID;size:255;uniqueIndex:idx_uid"`
	TenantID        string `gorm:"column:TenantID;size:255;uniqueIndex:idx_uid"`
}

// TableName 动态生成表名，格式: gvk_group_version_kind
func (dm *DynamicModel) TableName() string {
	name := strings.TrimSuffix(dm.Resource, "s")
	if len(name) > 0 {
		name = strings.ToUpper(name[:1]) + name[1:]
	}
	return name
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
