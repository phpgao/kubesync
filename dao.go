package main

import (
	"context"

	"gorm.io/gorm"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func NewDao(db *gorm.DB, modelFunc func(*unstructured.Unstructured) Unstructured, extraCondition map[string]any) ResourceStorage {
	//dsn := "root:fdj68Sbyj4S6QLf@tcp(9.134.107.122:3306)/cmdb?charset=utf8mb4&parseTime=True&loc=Local"
	//db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	//if err != nil {
	//	panic(err)
	//}
	return &Dao{
		db:             db,
		modelFunc:      modelFunc,
		extraCondition: extraCondition,
	}
}

type Dao struct {
	db             *gorm.DB
	modelFunc      func(*unstructured.Unstructured) Unstructured
	extraCondition map[string]any
	query          func(context.Context, *gorm.DB, *unstructured.Unstructured) *gorm.DB
	simpleQuery    func(context.Context, string, string) *gorm.DB
}

func (d Dao) AutoMigrate(ctx context.Context) error {
	model := d.modelFunc(nil)
	return d.db.Table(model.TableName()).AutoMigrate(&NamespacedDynamicModel{})

}

func (d Dao) Find(ctx context.Context, namespace string, name string) (*unstructured.Unstructured, error) {
	model := d.modelFunc(nil)
	err := d.query(ctx, d.db, nil).Where("namespace = ?", namespace).Where("name = ?", name).Find(model).Error
	if err != nil {
		return nil, err
	}
	return model.ToUnstructured()
}

func (d Dao) Create(ctx context.Context, object *unstructured.Unstructured) error {
	model := d.modelFunc(object)
	query := d.db.WithContext(ctx).Table(model.TableName())
	return query.Create(model).Error
}

func (d Dao) Save(ctx context.Context, object *unstructured.Unstructured) error {
	model := d.modelFunc(object)
	return d.query(ctx, d.db, object).Model(model).Updates(model).Error
}

func (d Dao) Delete(ctx context.Context, object *unstructured.Unstructured) error {
	model := d.modelFunc(object)
	return d.query(ctx, d.db, object).Delete(model).Error
}

func (d Dao) NeedUpdate(ctx context.Context, new *unstructured.Unstructured, old any) bool {
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
