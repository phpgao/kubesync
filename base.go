package main

import (
	"context"

	"gorm.io/gorm"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Base struct {
	gvr             *schema.GroupVersionResource
	namespaced      bool
	dependencies    []*schema.GroupVersionResource
	needUpdate      NeedUpdateFunc
	storage         []ResourceStorage
	onAdd           EventHandler
	onUpdate        EventHandler
	onDelete        EventHandler
	toModel         func(*unstructured.Unstructured) Unstructured
	extraConditions map[string]any
	query           func(context.Context) *gorm.DB
}

// Option 定义选项函数类型
type Option func(*Base)

// NewBase 创建默认Base实例并应用选项
func NewBase(gvr *schema.GroupVersionResource, namespaced bool, opts ...Option) *Base {
	b := &Base{
		gvr:        gvr,
		namespaced: namespaced,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// WithDependencies 选项函数：设置依赖项
func WithDependencies(dependencies ...*schema.GroupVersionResource) Option {
	return func(b *Base) {
		b.dependencies = append(b.dependencies, dependencies...)
	}
}

// WithNeedUpdate  选项函数：设置更新检查函数
func WithNeedUpdate(fn NeedUpdateFunc) Option {
	return func(b *Base) {
		b.needUpdate = fn
	}
}

func WithStorage(storage ...ResourceStorage) Option {
	return func(b *Base) {
		b.storage = append(b.storage, storage...)
	}
}

func WithOnAdd(fn EventHandler) Option {
	return func(b *Base) {
		b.onAdd = fn
	}
}

func WithOnUpdate(fn EventHandler) Option {
	return func(b *Base) {
		b.onUpdate = fn
	}
}

func WithOnDelete(fn EventHandler) Option {
	return func(b *Base) {
		b.onDelete = fn
	}
}

func WithToModel(fn func(*unstructured.Unstructured) Unstructured) Option {
	return func(b *Base) {
		b.toModel = fn
	}
}

func WithExtraConditions(extraConditions map[string]any) Option {
	return func(b *Base) {
		b.extraConditions = extraConditions
	}
}

func (b *Base) GetGVR() *schema.GroupVersionResource {
	return b.gvr
}

func (b *Base) GetDependencies() []*schema.GroupVersionResource {
	return b.dependencies
}

func (b *Base) GetNeedUpdate() NeedUpdateFunc {
	return b.needUpdate
}

func (b *Base) GetStorage() []ResourceStorage {
	return b.storage
}

func (b *Base) OnAdd(ctx context.Context, ctrl *Controller, obj *unstructured.Unstructured) error {
	return b.onAdd(ctx, ctrl, obj)
}

func (b *Base) OnUpdate(ctx context.Context, ctrl *Controller, obj *unstructured.Unstructured) error {
	return b.onUpdate(ctx, ctrl, obj)
}

func (b *Base) OnDelete(ctx context.Context, ctrl *Controller, obj *unstructured.Unstructured) error {
	return b.onDelete(ctx, ctrl, obj)
}

func (b *Base) GetNamespaced() bool {
	return b.namespaced
}

func (b *Base) ToModel() func(*unstructured.Unstructured) Unstructured {
	return func(obj *unstructured.Unstructured) Unstructured {
		if b.namespaced {
			if obj != nil {
				marshalJSON, err := obj.MarshalJSON()
				if err != nil {
					return nil
				}
				return &NamespacedDynamicModel{
					DynamicModel: DynamicModel{
						Name:      obj.GetName(),
						Raw:       string(marshalJSON),
						Version:   b.gvr.Version,
						Resource:  b.gvr.Resource,
						ClusterID: b.extraConditions["ClusterID"].(string),
						TenantID:  b.extraConditions["TenantID"].(string),
					},
					Namespace: obj.GetNamespace(),
				}
			}
			return &NamespacedDynamicModel{
				DynamicModel: DynamicModel{
					Version:   b.gvr.Version,
					Resource:  b.gvr.Resource,
					ClusterID: b.extraConditions["ClusterID"].(string),
					TenantID:  b.extraConditions["TenantID"].(string),
				},
			}
		}
		if obj != nil {
			marshalJSON, err := obj.MarshalJSON()
			if err != nil {
				return nil
			}
			return &DynamicModel{
				Name:      obj.GetName(),
				Raw:       string(marshalJSON),
				Version:   b.gvr.Version,
				Resource:  b.gvr.Resource,
				ClusterID: b.extraConditions["ClusterID"].(string),
				TenantID:  b.extraConditions["TenantID"].(string),
			}
		}
		return &DynamicModel{
			Version:   b.gvr.Version,
			Resource:  b.gvr.Resource,
			ClusterID: b.extraConditions["ClusterID"].(string),
			TenantID:  b.extraConditions["TenantID"].(string),
		}
	}
}

func (b *Base) GetExtraConditions() map[string]any {
	return b.extraConditions
}

func (b *Base) GetTableName() string {
	return b.gvr.Resource
}

func (b *Base) GetQuery() func(context.Context, *gorm.DB, *unstructured.Unstructured) *gorm.DB {
	return func(ctx context.Context, db *gorm.DB, obj *unstructured.Unstructured) *gorm.DB {
		query := db.WithContext(ctx).Table(b.GetTableName())
		if b.namespaced {
			query = query.Where("name = ?", obj.GetName()).Where("namespace = ?", obj.GetNamespace())
		} else {
			query = query.Where("name = ?", obj.GetName())
		}
		return query
	}
}

func (b *Base) GetSimpleQuery() func(context.Context, string, string) *gorm.DB {
	return func(ctx context.Context, namespace string, name string) *gorm.DB {
		query := b.query(ctx)
		if b.namespaced {
			query = query.Where("name = ?", name).Where("namespace = ?", namespace)
		} else {
			query = query.Where("name = ?", name)
		}
		return query
	}
}
