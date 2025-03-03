package main

import (
	"context"
	"fmt"
	"log"

	"gorm.io/gorm"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func NewDao(db *gorm.DB) ResourceStorage {
	//dsn := "root:fdj68Sbyj4S6QLf@tcp(9.134.107.122:3306)/cmdb?charset=utf8mb4&parseTime=True&loc=Local"
	//db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	//if err != nil {
	//	panic(err)
	//}
	return &Dao{
		db: db,
	}
}

type Dao struct {
	db *gorm.DB
}

func (d Dao) AutoMigrate(ctx context.Context, gvr schema.GroupVersionResource, namespaced bool) error {
	if namespaced {
		model := &NamespacedDynamicModel{
			DynamicModel: DynamicModel{
				Version:  gvr.Version,
				Resource: gvr.Resource,
			},
		}
		// 自动迁移表结构
		err := d.db.Table(model.TableName()).AutoMigrate(&NamespacedDynamicModel{})
		if err != nil {
			log.Println(gvr.String())
			return fmt.Errorf("auto migrate failed: %v", err)
		}
		return nil
	} else {
		model := &DynamicModel{
			Version:  gvr.Version,
			Resource: gvr.Resource,
		}
		// 自动迁移表结构
		err := d.db.Table(model.TableName()).AutoMigrate(&DynamicModel{})
		if err != nil {
			log.Println(gvr.String())
			return fmt.Errorf("auto migrate failed: %v", err)
		}
		return nil
	}
}

func (d Dao) Find(ctx context.Context, resource schema.GroupVersionResource, namespaced bool, namespace string, name string) (*unstructured.Unstructured, error) {
	model := getModel(resource, namespaced, nil)
	query := d.db.WithContext(ctx).Table(model.TableName()).Where("name = ?", name)
	if namespaced {
		query = query.Where("namespace = ?", namespace)
	}

	err := query.Find(model).Error
	if err != nil {
		return nil, err
	}
	return model.ToUnstructured()
}

func (d Dao) Create(ctx context.Context, resource schema.GroupVersionResource, namespaced bool, object *unstructured.Unstructured) error {
	model := getModel(resource, namespaced, object)
	query := d.db.WithContext(ctx).Table(model.TableName()).Where("name = ?", object.GetName())
	if namespaced {
		query = query.Where("namespace = ?", object.GetNamespace())
	}

	return query.Create(model).Error
}

func (d Dao) Save(ctx context.Context, resource schema.GroupVersionResource, namespaced bool, object *unstructured.Unstructured) error {
	model := getModel(resource, namespaced, object)
	query := d.db.WithContext(ctx).Table(model.TableName()).Where("name = ?", object.GetName())
	if namespaced {
		query = query.Where("namespace = ?", object.GetNamespace())
	}

	return query.Model(model).Updates(model).Error
}

func (d Dao) Delete(ctx context.Context, resource schema.GroupVersionResource, namespaced bool, object *unstructured.Unstructured) error {
	model := getModel(resource, namespaced, object)
	query := d.db.WithContext(ctx).Table(model.TableName()).Where("name = ?", object.GetName())
	if namespaced {
		query = query.Where("namespace = ?", object.GetNamespace())
	}

	return query.Model(model).Delete(model).Error
}

func (d Dao) NeedUpdate(ctx context.Context, gvr schema.GroupVersionResource, namespaced bool, new *unstructured.Unstructured, old *unstructured.Unstructured) bool {
	return true
}

func getModel(gvr schema.GroupVersionResource, namespaced bool, object *unstructured.Unstructured) Unstructured {
	if namespaced {
		if object != nil {
			marshalJSON, err := object.MarshalJSON()
			if err != nil {
				return nil
			}
			return &NamespacedDynamicModel{
				DynamicModel: DynamicModel{
					Name:     object.GetName(),
					Raw:      string(marshalJSON),
					Version:  gvr.Version,
					Resource: gvr.Resource,
				},
				Namespace: object.GetNamespace(),
			}
		}
		return &NamespacedDynamicModel{
			DynamicModel: DynamicModel{
				Version:  gvr.Version,
				Resource: gvr.Resource,
			},
		}
	}
	if object != nil {
		marshalJSON, err := object.MarshalJSON()
		if err != nil {
			return nil
		}
		return &DynamicModel{
			Name:     object.GetName(),
			Raw:      string(marshalJSON),
			Version:  gvr.Version,
			Resource: gvr.Resource,
		}
	}
	return &DynamicModel{
		Version:  gvr.Version,
		Resource: gvr.Resource,
	}
}
