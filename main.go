package main

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

const (
	defaultResyncPeriod = 30 * time.Second
	workerCount         = 2
)

type (
	HandlerFunc    func(context.Context, *Controller, string, *unstructured.Unstructured) error
	NeedUpdateFunc func(*unstructured.Unstructured, *unstructured.Unstructured) bool
	UniqueFunc     func(*unstructured.Unstructured, bool) string
)

func main() {
	kubeconfig := "/Users/jimmygao/.kube/config"

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		klog.Fatal(err)
	}

	manager := NewControllerManager(config)

	manager.RegisterHandler(CoreV1Pod, func(ctx context.Context, ctrl *Controller, action string, obj *unstructured.Unstructured) error {
		// 将 Unstructured 转换为 Pod 对象
		pod := &v1.Pod{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, pod)
		if err != nil {
			return fmt.Errorf("转换 Pod 失败: %v", err)
		}

		// 现在可以使用结构化的 Pod 对象
		fmt.Printf("处理 Pod: %s/%s\n", pod.Namespace, pod.Name)
		fmt.Printf("节点: %s\n", pod.Spec.NodeName)
		fmt.Printf("阶段: %s\n", pod.Status.Phase)

		// 示例：打印容器信息
		for _, container := range pod.Spec.Containers {
			fmt.Printf("容器 %s 使用镜像: %s\n", container.Name, container.Image)
		}

		for _, own := range pod.OwnerReferences {
			o, err := ctrl.GetObj(AppsV1ReplicaSet, pod.Namespace, own.Name)
			if err != nil {
				fmt.Printf("获取 Deployment 失败: %v\n", err)
				return err
			}
			fmt.Printf("%s\n", o.GetName())
		}

		return nil
	},
	)

	manager.AddDependency(CoreV1Pod, []schema.GroupVersionResource{AppsV1Deployment, AppsV1ReplicaSet})
	//manager.AddDependency(AppsV1Deployment, []schema.GroupVersionResource{CoreV1Pod})

	manager.RegisterBlacklist(FlowSchemaV1Beta3)
	manager.RegisterBlacklist(PriorityLevelConfigV1Beta3)
	manager.RegisterBlacklist(LeasesV1)
	manager.RegisterBlacklist(CoreV1Event)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	manager.AddDefaultDao(NewDao(db))

	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		klog.Fatal(err)
	}
}
