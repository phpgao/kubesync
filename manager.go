package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

type ControllerManager struct {
	clusterID     string
	mu            sync.Mutex
	config        *rest.Config
	dynamicClient dynamic.Interface
	kubeClient    kubernetes.Interface
	controllers   map[schema.GroupVersionResource]*Controller
	handlerMap    sync.Map
	needUpdateMap sync.Map
	whitelist     map[schema.GroupVersionResource]struct{}
	whitelistMu   sync.RWMutex
	dependencyMap map[schema.GroupVersionResource][]schema.GroupVersionResource
	dependencyMu  sync.RWMutex
	daoMap        map[schema.GroupVersionResource][]Dao
	daoMu         sync.RWMutex
	defaultDao    []Dao
	InClusterMode bool
}

func NewControllerManager(clusterID string, config *rest.Config) *ControllerManager {
	return &ControllerManager{
		clusterID:     clusterID,
		config:        config,
		controllers:   make(map[schema.GroupVersionResource]*Controller),
		handlerMap:    sync.Map{},
		needUpdateMap: sync.Map{},
		whitelist:     make(map[schema.GroupVersionResource]struct{}),
		dependencyMap: make(map[schema.GroupVersionResource][]schema.GroupVersionResource),
	}
}

func (cm *ControllerManager) GetController(gvr schema.GroupVersionResource) *Controller {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if ctrl, ok := cm.controllers[gvr]; ok {
		return ctrl
	}
	return nil
}

func (cm *ControllerManager) AddDependency(gvr schema.GroupVersionResource, dependencies []schema.GroupVersionResource) {
	cm.dependencyMu.Lock()
	defer cm.dependencyMu.Unlock()

	// 创建临时依赖副本用于循环检测
	tempDeps := cloneDependencyMap(cm.dependencyMap)
	tempDeps[gvr] = append(tempDeps[gvr], dependencies...)

	if HasCycle(tempDeps) {
		panic(fmt.Errorf("检测到循环依赖: %v -> %v", gvr, dependencies))
	}
	cm.dependencyMap[gvr] = append(cm.dependencyMap[gvr], dependencies...)
}

// 新增深拷贝函数
func cloneDependencyMap(original map[schema.GroupVersionResource][]schema.GroupVersionResource) map[schema.GroupVersionResource][]schema.GroupVersionResource {
	clone := make(map[schema.GroupVersionResource][]schema.GroupVersionResource, len(original))
	for k, v := range original {
		// 对切片进行深拷贝
		clone[k] = append([]schema.GroupVersionResource(nil), v...)
	}
	return clone
}

func (cm *ControllerManager) GetDependency(gvr schema.GroupVersionResource) []schema.GroupVersionResource {
	cm.dependencyMu.RLock()
	defer cm.dependencyMu.RUnlock()
	return cm.dependencyMap[gvr]
}

func (cm *ControllerManager) RegisterNeedUpdate(gvr schema.GroupVersionResource, handler NeedUpdateFunc) {
	cm.handlerMap.Store(gvr, handler)
}

func (cm *ControllerManager) Start(ctx context.Context) error {
	var err error
	cm.kubeClient, err = kubernetes.NewForConfig(cm.config)
	if err != nil {
		return err
	}

	cm.dynamicClient, err = dynamic.NewForConfig(cm.config)
	if err != nil {
		return err
	}

	discoveryClient := cm.kubeClient.Discovery()
	_, resourceLists, err := discoveryClient.ServerGroupsAndResources()
	if err != nil {
		return err
	}

	for _, resourceList := range resourceLists {
		var gv schema.GroupVersion
		gv, err = schema.ParseGroupVersion(resourceList.GroupVersion)
		if err != nil {
			continue
		}

		for _, resource := range resourceList.APIResources {
			// 跳过子资源（包含/的resource名称）
			if strings.Contains(resource.Name, "/") {
				continue
			}
			gvr := gv.WithResource(resource.Name)
			// 黑名单检查
			log.Printf("check blacklist for %s\n", gvr.String())
			if !cm.isWhitelisted(gvr) {
				log.Printf("Skipping blacklisted GVR: %s\n", gvr)
				continue
			}
			// 检查是否支持list操作
			if !stringSliceContains(resource.Verbs, "list") {
				continue
			}
			// 检查是否支持watch操作
			if !stringSliceContains(resource.Verbs, "watch") {
				continue
			}
			cm.createControllerForGVR(gvr, resource.Namespaced)

		}
	}

	// Start leader election
	go cm.runLeaderElection(ctx)

	<-ctx.Done()
	return nil
}

func (cm *ControllerManager) createControllerForGVR(gvr schema.GroupVersionResource, namespaced bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, exists := cm.controllers[gvr]; exists {
		return
	}

	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		cm.dynamicClient,
		defaultResyncPeriod,
		metav1.NamespaceAll,
		nil,
	)

	informer := factory.ForResource(gvr)
	queue := workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[string](),
		workqueue.TypedRateLimitingQueueConfig[string]{
			Name: gvr.String(),
		},
	)

	unit := NewBase(cm.clusterID, gvr, namespaced, WithStorage(cm.GetDao(gvr, namespaced)))

	ctrl := &Controller{
		cm:         cm,
		name:       gvr.String(),
		gvr:        gvr,
		namespaced: namespaced,
		informer:   informer,
		lister:     informer.Lister(),
		queue:      queue,
		dependency: cm.GetDependency(gvr),
		unit:       unit,
		clusterID:  cm.clusterID,
	}

	_, err := informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ctrl.onAdd,
		UpdateFunc: ctrl.onUpdate,
		DeleteFunc: ctrl.onDelete,
	})
	if err != nil {
		return
	}

	cm.controllers[gvr] = ctrl
}

func (cm *ControllerManager) runLeaderElection(ctx context.Context) {
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      "dynamic-controller-leader",
			Namespace: "default",
		},
		Client: cm.kubeClient.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			// Identity: os.Getenv("POD_NAME"),
			Identity: "dynamic-controller-leader",
		},
	}

	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   15 * time.Second,
		RenewDeadline:   10 * time.Second,
		RetryPeriod:     2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				klog.Info("Started leading")
				cm.startControllers(ctx)
			},
			OnStoppedLeading: func() {
				klog.Info("Stopped leading")
			},
		},
	})
}

// startControllers 启动控制器
func (cm *ControllerManager) startControllers(ctx context.Context) {
	for gvr, ctrl := range cm.controllers {
		klog.Infof("Starting controller for %s", gvr)
		go ctrl.Run(ctx)
	}
}

// RegisterWhitelist 添加白名单
func (cm *ControllerManager) RegisterWhitelist(gvr schema.GroupVersionResource) {
	log.Printf("Registering blacklist for %s\n", gvr.String())
	cm.whitelistMu.Lock()
	defer cm.whitelistMu.Unlock()
	cm.whitelist[gvr] = struct{}{}
}

// isWhitelisted 检查是否在白名单中
func (cm *ControllerManager) isWhitelisted(gvr schema.GroupVersionResource) bool {
	cm.whitelistMu.RLock()
	defer cm.whitelistMu.RUnlock()
	_, ok := cm.whitelist[gvr]
	return ok
}

func (cm *ControllerManager) GetDao(gvr schema.GroupVersionResource, namespaced bool) Dao {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	if gvr == CoreV1Pod {
		return NewDao(cm.clusterID, db.Debug(), gvr, namespaced, func(ctx context.Context, model *DynamicModel, obj *unstructured.Unstructured) BaseModel {
			if obj == nil {
				return &Pod{
					DynamicModel: *model,
					Phase:        "",
				}
			}
			pod := &v1.Pod{}
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, pod)
			if err != nil {
				log.Printf(err.Error())
			}
			podModel := &Pod{
				DynamicModel: *model,
				Phase:        string(pod.Status.Phase),
			}

			return podModel
		})
	}

	return NewDao(cm.clusterID, db.Debug(), gvr, namespaced, nil)
}

type Pod struct {
	DynamicModel `gorm:"embedded"`
	Phase        string `gorm:"column:Phase"`
}
