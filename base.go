package main

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Base struct {
	gvr             schema.GroupVersionResource
	namespaced      bool
	needUpdate      NeedUpdateFunc
	storage         []Dao
	onAdd           EventHandler
	onUpdate        EventHandler
	onDelete        EventDeleteHandler
	toModel         func(*unstructured.Unstructured) BaseModel
	extraConditions map[string]any
	query           func(context.Context) *gorm.DB
	setExtraFields  func(context.Context, *DynamicModel)
}

// Option 定义选项函数类型
type Option func(*Base)

// NewBase 创建默认Base实例并应用选项
func NewBase(gvr schema.GroupVersionResource, namespaced bool, opts ...Option) Unit {
	b := &Base{
		gvr:        gvr,
		namespaced: namespaced,
		onAdd:      DefaultAddFN,
		onUpdate:   DefaultAddFN,
		onDelete:   DefaultDeleteFN,
		needUpdate: DefaultNeedUpdateFN,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

var DefaultNeedUpdateFN = DefaultNeedUpdate

func DefaultNeedUpdate(old *unstructured.Unstructured, new *unstructured.Unstructured) bool {
	return old.GetResourceVersion() != new.GetResourceVersion()
}

// WithNeedUpdate  选项函数：设置更新检查函数
func WithNeedUpdate(fn NeedUpdateFunc) Option {
	return func(b *Base) {
		b.needUpdate = fn
	}
}

func WithStorage(storage ...Dao) Option {
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

func WithOnDelete(fn EventDeleteHandler) Option {
	return func(b *Base) {
		b.onDelete = fn
	}
}

func WithToModel(fn func(*unstructured.Unstructured) BaseModel) Option {
	return func(b *Base) {
		b.toModel = fn
	}
}

func WithExtraConditions(extraConditions map[string]any) Option {
	return func(b *Base) {
		b.extraConditions = extraConditions
	}
}

func (b *Base) GetGVR() schema.GroupVersionResource {
	return b.gvr
}

func (b *Base) GetNeedUpdate(old *unstructured.Unstructured, new *unstructured.Unstructured) bool {
	return b.needUpdate(old, new)
}

func (b *Base) GetStorage() []Dao {
	return b.storage
}

func (b *Base) OnAdd(ctx context.Context, ctrl *Controller, obj *unstructured.Unstructured) error {
	return b.onAdd(ctx, ctrl, b.storage, obj)
}

func (b *Base) OnUpdate(ctx context.Context, ctrl *Controller, obj *unstructured.Unstructured) error {
	return b.onUpdate(ctx, ctrl, b.storage, obj)
}

func (b *Base) OnDelete(ctx context.Context, storage []Dao, namespace, name string) error {
	return b.onDelete(ctx, storage, namespace, name)
}

func (b *Base) GetNamespaced() bool {
	return b.namespaced
}

var DefaultDeleteFN = DefaultDelete

func DefaultDelete(ctx context.Context, storages []Dao, namespace, name string) error {
	for _, storage := range storages {
		err := storage.Delete(ctx, namespace, name)
		if err != nil {
			return err
		}
	}
	return nil
}

var DefaultAddFN = DefaultAdd

func DefaultAdd(ctx context.Context, ctrl *Controller, storages []Dao, obj *unstructured.Unstructured) error {
	for _, storage := range storages {
		// 查询是否存在
		model, err := storage.Find(ctx, obj.GetNamespace(), obj.GetName())
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// 不存在则创建
				err = storage.Create(ctx, obj)
				if err != nil {
				}
				continue
			}
			continue
		}
		if storage.NeedUpdate(ctx, obj, model) {
			err = storage.Save(ctx, obj)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

type Unit interface {
	GetGVR() schema.GroupVersionResource
	GetNeedUpdate(*unstructured.Unstructured, *unstructured.Unstructured) bool
	GetStorage() []Dao
	OnAdd(ctx context.Context, ctrl *Controller, obj *unstructured.Unstructured) error
	OnUpdate(ctx context.Context, ctrl *Controller, obj *unstructured.Unstructured) error
	OnDelete(ctx context.Context, storage []Dao, namespace, name string) error
	GetNamespaced() bool
}
