package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"time"

	"gorm.io/gorm"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Dao interface {
	AutoMigrate(context.Context) error
	Find(context.Context, string, string) (BaseModel, error)
	Save(context.Context, *unstructured.Unstructured) error
	Create(context.Context, *unstructured.Unstructured) error
	Delete(context.Context, string, string) error
	// NeedUpdate returns true if the object needs to be updated
	// It's Useful for update new field
	// in this function you need to compare the new object and the old data in db
	// if the new object is different from the old data in db, return true
	NeedUpdate(ctx context.Context, new *unstructured.Unstructured, old any) bool
}

func NewDao(db *gorm.DB, gvr schema.GroupVersionResource, namespaced bool, extraFields map[string]any, realModelFn func(ctx context.Context, model *DynamicModel, obj *unstructured.Unstructured) BaseModel) Dao {
	return &dao{
		db:          db,
		gvr:         gvr,
		namespaced:  namespaced,
		extraFields: extraFields,
		realModelFn: realModelFn,
	}
}

type dao struct {
	db          *gorm.DB
	gvr         schema.GroupVersionResource
	namespaced  bool
	extraFields map[string]any
	realModelFn func(ctx context.Context, model *DynamicModel, obj *unstructured.Unstructured) BaseModel
}

func (d *dao) GetModel(ctx context.Context, obj *unstructured.Unstructured) BaseModel {
	var (
		name            string
		namespace       string
		raw             string
		resourceVersion string
		uid             string
		labels          string
		annotations     string
		createAt        time.Time
	)

	if obj != nil {
		marshalJSON, err := obj.MarshalJSON()
		if err != nil {
			return nil
		}
		name = obj.GetName()
		namespace = obj.GetNamespace()
		resourceVersion = obj.GetResourceVersion()
		if labelsMap := obj.GetLabels(); labelsMap != nil {
			labels = MustJson(labelsMap)
		}
		if annotationsMap := obj.GetAnnotations(); annotationsMap != nil {
			annotations = MustJson(annotationsMap)
		}

		obj.GetCreationTimestamp()
		createAt = obj.GetCreationTimestamp().Time
		uid = string(obj.GetUID())
		raw = string(marshalJSON)
	}

	baseModel := DynamicModel{
		Model: gorm.Model{
			CreatedAt: createAt,
		},
		Name:            name,
		NameSpace:       namespace,
		Labels:          labels,
		Annotations:     annotations,
		Raw:             raw,
		Version:         d.gvr.Version,
		Resource:        d.gvr.Resource,
		UID:             uid,
		ResourceVersion: resourceVersion,
	}

	// 使用反射设置extraFields
	if len(d.extraFields) > 0 {
		modelValue := reflect.ValueOf(&baseModel).Elem()
		modelType := modelValue.Type()

		for i := 0; i < modelValue.NumField(); i++ {
			fieldName := modelType.Field(i).Name
			if value, exists := d.extraFields[fieldName]; exists {
				fieldValue := modelValue.FieldByName(fieldName)
				if fieldValue.IsValid() && fieldValue.CanSet() {
					val := reflect.ValueOf(value)
					if val.Type().AssignableTo(fieldValue.Type()) {
						fieldValue.Set(val)
					}
				}
			}
		}
	}

	if d.realModelFn != nil {
		log.Printf("realModelFn")
		return d.realModelFn(ctx, &baseModel, obj)
	}

	return &baseModel
}

func MustJson(data any) string {
	marshal, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return string(marshal)
}

func (d *dao) AutoMigrate(ctx context.Context) error {
	model := d.GetModel(ctx, nil)
	log.Println(MustJson(model))
	return d.db.Table(model.TableName()).AutoMigrate(d.GetModel(ctx, nil))
}

func (d *dao) TableName(ctx context.Context) string {
	return d.GetModel(ctx, nil).TableName()
}

func (d *dao) GetWhere(ctx context.Context, namespace string, name string) *gorm.DB {
	query := d.db.WithContext(ctx).Table(d.TableName(ctx)).Where("name = ?", name)
	if d.namespaced {
		query = query.Where("namespace = ?", namespace)
	}
	if d.extraFields != nil {
		query = query.Where(d.extraFields)
	}
	return query
}

func (d *dao) Find(ctx context.Context, namespace string, name string) (BaseModel, error) {
	model := d.GetModel(ctx, nil)
	query := d.GetWhere(ctx, namespace, name)
	return model, query.First(model).Error
}

func (d *dao) Save(ctx context.Context, u *unstructured.Unstructured) error {
	model := d.GetModel(ctx, u)
	query := d.GetWhere(ctx, u.GetNamespace(), u.GetName())
	return query.Updates(model).Error
}

func (d *dao) Create(ctx context.Context, u *unstructured.Unstructured) error {
	model := d.GetModel(ctx, u)
	return d.db.WithContext(ctx).Table(d.TableName(ctx)).Create(model).Error
}

func (d *dao) Delete(ctx context.Context, namespace string, name string) error {
	model := d.GetModel(ctx, nil)
	query := d.GetWhere(ctx, namespace, name)
	return query.Delete(model).Error
}

func (d *dao) NeedUpdate(ctx context.Context, new *unstructured.Unstructured, old any) bool {
	return true
}
