package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	apiserror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

type Controller struct {
	name       string
	clusterID  string
	gvr        schema.GroupVersionResource
	namespaced bool
	informer   informers.GenericInformer
	lister     cache.GenericLister
	queue      workqueue.TypedRateLimitingInterface[string]
	cm         *ControllerManager
	dependency []schema.GroupVersionResource
	ready      bool
	unit       Unit
}

func generateKey(action string, obj metav1.Object) string {
	return action + "/" + obj.GetNamespace() + "/" + obj.GetName()
}

func parseKey(key string) (string, string, string) {
	parts := strings.Split(key, "/")
	if len(parts) != 3 {
		return "", "", ""
	}
	return parts[0], parts[1], parts[2]
}

func (c *Controller) Namespaced() bool {
	return c.namespaced
}

func (c *Controller) GetLister() cache.GenericLister {
	return c.lister
}
func (c *Controller) GetInformer() informers.GenericInformer {
	return c.informer
}

func (c *Controller) GetGVR() schema.GroupVersionResource {
	return c.gvr
}

func (c *Controller) GetObj(gvr schema.GroupVersionResource, namespace, name string) (*unstructured.Unstructured, error) {
	controller := c.cm.GetController(gvr)
	if controller == nil {
		return nil, fmt.Errorf("controller not found")
	}

	if !controller.ready {
		return nil, fmt.Errorf("controller not ready")
	}
	lister := controller.GetLister()
	if controller.Namespaced() {
		ojb, err := lister.ByNamespace(namespace).Get(name)
		if err != nil {
			return nil, err
		}
		return ojb.(*unstructured.Unstructured), nil
	}
	ojb, err := lister.Get(name)
	if err != nil {
		return nil, err
	}
	return ojb.(*unstructured.Unstructured), nil
}

func (c *Controller) onAdd(obj interface{}) {
	uObj := obj.(*unstructured.Unstructured)
	log.Println(c.name + generateKey(ActionAdd, uObj))
	c.queue.Add(generateKey(ActionAdd, uObj))
}

func (c *Controller) onUpdate(oldObj, newObj interface{}) {
	newObject := newObj.(*unstructured.Unstructured)
	oldObject := oldObj.(*unstructured.Unstructured)
	if !c.unit.GetNeedUpdate(oldObject, newObject) {
		return
	}
	log.Println(c.name + generateKey(ActionUpdate, newObject))
	c.queue.Add(generateKey(ActionUpdate, newObject))
}

func (c *Controller) onDelete(obj interface{}) {
	if deleted, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = deleted.Obj
	}
	uObj := obj.(*unstructured.Unstructured)
	log.Println(c.name + generateKey(ActionDelete, uObj))
	c.queue.Add(generateKey(ActionDelete, uObj))
}

func (c *Controller) Run(ctx context.Context) {

	for _, storage := range c.unit.GetStorage() {

		err := storage.AutoMigrate(ctx)
		if err != nil {
			log.Println(err)
			continue
		}
	}

	defer c.queue.ShutDown()

	stopCh := ctx.Done()

	go c.informer.Informer().Run(stopCh)
	hasSynced := []cache.InformerSynced{c.informer.Informer().HasSynced}
	for _, gvr := range c.dependency {
		hasSynced = append(hasSynced, c.cm.GetController(gvr).GetInformer().Informer().HasSynced)
	}
	if !cache.WaitForCacheSync(stopCh, hasSynced...) {
		klog.Error("Timed out waiting for caches to sync")
		return
	}
	c.ready = true
	fmt.Printf("Controller %s is ready\n", c.name)
	for i := 0; i < workerCount; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
	}
}

func (c *Controller) processNextItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)
	// 解析队列键
	action, namespace, name := parseKey(key)
	if action == "" || name == "" {
		c.queue.Forget(key)
		return true
	}
	var obj runtime.Object
	var err error
	if c.namespaced {
		obj, err = c.lister.ByNamespace(namespace).Get(name)
	} else {
		obj, err = c.lister.Get(name)
	}
	log.Println(key)
	if err != nil {
		if apiserror.IsNotFound(err) {
			action = ActionDelete
		} else {
			c.queue.AddRateLimited(key)
			return true
		}
	}
	ctx := context.Background()
	switch action {
	case ActionAdd:
		err = c.unit.OnAdd(ctx, c, obj.(*unstructured.Unstructured))
	case ActionUpdate:
		err = c.unit.OnUpdate(ctx, c, obj.(*unstructured.Unstructured))
	case ActionDelete:
		err = c.unit.OnDelete(ctx, c.unit.GetStorage(), namespace, name)
	default:
		err = fmt.Errorf("unknown action: %s", action)
	}
	if err != nil {
		c.queue.AddRateLimited(key)
		return true
	}

	c.queue.Forget(key)
	return true
}
