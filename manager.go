package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	mu            sync.Mutex
	config        *rest.Config
	dynamicClient dynamic.Interface
	kubeClient    kubernetes.Interface
	controllers   map[schema.GroupVersionResource]*Controller
	handlerMap    sync.Map
	needUpdateMap sync.Map
	uniqueFuncMap sync.Map
	blacklist     map[schema.GroupVersionResource]struct{}
	blacklistMu   sync.RWMutex
	dependencyMap map[schema.GroupVersionResource][]schema.GroupVersionResource
	dependencyMu  sync.RWMutex
	daoMap        map[schema.GroupVersionResource][]ResourceStorage
	daoMu         sync.RWMutex
	defaultDao    []ResourceStorage
}

func NewControllerManager(config *rest.Config) *ControllerManager {
	return &ControllerManager{
		config:        config,
		controllers:   make(map[schema.GroupVersionResource]*Controller),
		handlerMap:    sync.Map{},
		needUpdateMap: sync.Map{},
		blacklist:     make(map[schema.GroupVersionResource]struct{}),
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

func (cm *ControllerManager) RegisterHandler(gvr schema.GroupVersionResource, handler HandlerFunc) {
	cm.handlerMap.Store(gvr, handler)
}

func (cm *ControllerManager) RegisterDao(gvr schema.GroupVersionResource, dbHandler []ResourceStorage) {
	cm.daoMu.Lock()
	defer cm.daoMu.Unlock()
	cm.daoMap[gvr] = append(cm.daoMap[gvr], dbHandler...)
}

func (cm *ControllerManager) GetDao(gvr schema.GroupVersionResource) []ResourceStorage {
	cm.daoMu.Lock()
	defer cm.daoMu.Unlock()
	if handler, ok := cm.daoMap[gvr]; ok {
		return handler
	}
	return cm.getDefaultDao()
}

func (cm *ControllerManager) getDefaultDao() []ResourceStorage {
	return cm.defaultDao
}

func (cm *ControllerManager) AddDefaultDao(dbHandler ResourceStorage) {
	cm.defaultDao = append(cm.defaultDao, dbHandler)
}

func (cm *ControllerManager) GetHandler(gvr schema.GroupVersionResource) HandlerFunc {
	if handler, ok := cm.handlerMap.Load(gvr); ok {
		return handler.(HandlerFunc)
	}
	return DefaultHandler
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

func (cm *ControllerManager) RegisterUniqueFunc(gvr schema.GroupVersionResource, handler UniqueFunc) {
	cm.uniqueFuncMap.Store(gvr, handler)
}

func (cm *ControllerManager) GetUniqueFunc(gvr schema.GroupVersionResource) UniqueFunc {
	if handler, ok := cm.uniqueFuncMap.Load(gvr); ok {
		return handler.(UniqueFunc)
	}
	return DefaultUniqueFunc
}

func (cm *ControllerManager) GetNeedUpdate(gvr schema.GroupVersionResource) NeedUpdateFunc {
	if handler, ok := cm.needUpdateMap.Load(gvr); ok {
		return handler.(NeedUpdateFunc)
	}
	return DefaultNeedUpdate
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
			if cm.isBlacklisted(gvr) {
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

	handler := cm.GetHandler(gvr)
	needUpdate := cm.GetNeedUpdate(gvr)

	ctrl := &Controller{
		cm:         cm,
		name:       gvr.String(),
		gvr:        gvr,
		namespaced: namespaced,
		informer:   informer,
		lister:     informer.Lister(),
		queue:      queue,
		handler:    handler,
		needUpdate: needUpdate,
		dependency: cm.GetDependency(gvr),
		dao:        cm.getDefaultDao(),
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

// RegisterBlacklist 添加黑名单注册方法
func (cm *ControllerManager) RegisterBlacklist(gvr schema.GroupVersionResource) {
	log.Printf("Registering blacklist for %s\n", gvr.String())
	cm.blacklistMu.Lock()
	defer cm.blacklistMu.Unlock()
	cm.blacklist[gvr] = struct{}{}
}

// isBlacklisted 检查是否在黑名单中
func (cm *ControllerManager) isBlacklisted(gvr schema.GroupVersionResource) bool {
	cm.blacklistMu.RLock()
	defer cm.blacklistMu.RUnlock()
	_, ok := cm.blacklist[gvr]
	return ok
}
