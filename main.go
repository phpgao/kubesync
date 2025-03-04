package main

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

const (
	defaultResyncPeriod = 30 * time.Second
	workerCount         = 10
)

type (
	NeedUpdateFunc     func(*unstructured.Unstructured, *unstructured.Unstructured) bool
	EventHandler       func(ctx context.Context, ctrl *Controller, storage []Dao, obj *unstructured.Unstructured) error
	EventDeleteHandler func(context.Context, []Dao, string, string) error
)

func main() {
	kubeconfig := "/Users/jimmygao/.kube/config"

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		klog.Fatal(err)
	}

	manager := NewControllerManager("cls-test", config)

	manager.AddDependency(CoreV1Pod, []schema.GroupVersionResource{AppsV1Deployment, AppsV1ReplicaSet})
	//manager.AddDependency(AppsV1Deployment, []schema.GroupVersionResource{CoreV1Pod})

	manager.RegisterWhitelist(AppsV1Deployment)
	manager.RegisterWhitelist(AppsV1ReplicaSet)
	manager.RegisterWhitelist(CoreV1Pod)
	manager.RegisterWhitelist(AppsV1StatefulSet)
	manager.RegisterWhitelist(AppsV1ControllerRevision)
	manager.RegisterWhitelist(AppsV1DaemonSet)
	manager.RegisterWhitelist(BatchV1Job)
	manager.RegisterWhitelist(BatchV1CronJob)
	manager.RegisterWhitelist(CoreV1Service)
	manager.RegisterWhitelist(CoreV1ConfigMap)
	manager.RegisterWhitelist(CoreV1Secret)
	manager.RegisterWhitelist(CoreV1Namespace)
	manager.RegisterWhitelist(CoreV1PersistentVolume)
	manager.RegisterWhitelist(CoreV1PersistentVolumeClaim)
	manager.RegisterWhitelist(NetworkingV1Ingress)
	manager.RegisterWhitelist(NetworkingV1IngressClass)
	manager.RegisterWhitelist(StorageV1StorageClass)

	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		klog.Fatal(err)
	}
}
