package main

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	ActionAdd    = "add"
	ActionUpdate = "update"
	ActionDelete = "delete"
)

// ======================== Core API ========================
var (
	CoreV1Pod = schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}

	CoreV1Service = schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "services",
	}

	CoreV1ConfigMap = schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}

	CoreV1Secret = schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "secrets",
	}

	CoreV1Namespace = schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "namespaces",
	}

	CoreV1Event = schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "events",
	}

	CoreV1PersistentVolume = schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "persistentvolumes",
	}

	CoreV1PersistentVolumeClaim = schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "persistentvolumeclaims",
	}
)

// ======================== Apps ========================
var (
	AppsV1Deployment = schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}

	AppsV1StatefulSet = schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "statefulsets",
	}

	AppsV1DaemonSet = schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "daemonsets",
	}

	AppsV1ReplicaSet = schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "replicasets",
	}
)

// ======================== Batch ========================
var (
	BatchV1Job = schema.GroupVersionResource{
		Group:    "batch",
		Version:  "v1",
		Resource: "jobs",
	}

	BatchV1CronJob = schema.GroupVersionResource{
		Group:    "batch",
		Version:  "v1",
		Resource: "cronjobs",
	}
)

// ======================== Networking ========================
var (
	NetworkingV1Ingress = schema.GroupVersionResource{
		Group:    "networking.k8s.io",
		Version:  "v1",
		Resource: "ingresses",
	}

	NetworkingV1IngressClass = schema.GroupVersionResource{
		Group:    "networking.k8s.io",
		Version:  "v1",
		Resource: "ingressclasses",
	}
)

// ======================== Storage ========================
var (
	StorageV1StorageClass = schema.GroupVersionResource{
		Group:    "storage.k8s.io",
		Version:  "v1",
		Resource: "storageclasses",
	}
)

// ======================== RBAC ========================
var (
	RbacV1Role = schema.GroupVersionResource{
		Group:    "rbac.authorization.k8s.io",
		Version:  "v1",
		Resource: "roles",
	}

	RbacV1RoleBinding = schema.GroupVersionResource{
		Group:    "rbac.authorization.k8s.io",
		Version:  "v1",
		Resource: "rolebindings",
	}

	RbacV1ClusterRole = schema.GroupVersionResource{
		Group:    "rbac.authorization.k8s.io",
		Version:  "v1",
		Resource: "clusterroles",
	}

	RbacV1ClusterRoleBinding = schema.GroupVersionResource{
		Group:    "rbac.authorization.k8s.io",
		Version:  "v1",
		Resource: "clusterrolebindings",
	}
)

// ======================== Autoscaling ========================
var (
	AutoscalingV2HorizontalPodAutoscaler = schema.GroupVersionResource{
		Group:    "autoscaling",
		Version:  "v2",
		Resource: "horizontalpodautoscalers",
	}
)

// ======================== CustomResourceDefinition ========================
var (
	ApiextensionsV1CRD = schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}
)

// ======================== Certificates ========================
var (
	CertificatesV1CertificateSigningRequest = schema.GroupVersionResource{
		Group:    "certificates.k8s.io",
		Version:  "v1",
		Resource: "certificatesigningrequests",
	}
)

// ======================== Scheduling ========================
var (
	SchedulingV1PriorityClass = schema.GroupVersionResource{
		Group:    "scheduling.k8s.io",
		Version:  "v1",
		Resource: "priorityclasses",
	}
)

// ======================== FlowControl ========================
var (
	FlowSchemaV1Beta3 = schema.GroupVersionResource{
		Group:    "flowcontrol.apiserver.k8s.io",
		Version:  "v1beta3",
		Resource: "flowschemas",
	}

	PriorityLevelConfigV1Beta3 = schema.GroupVersionResource{
		Group:    "flowcontrol.apiserver.k8s.io",
		Version:  "v1beta3",
		Resource: "prioritylevelconfigurations",
	}
)

// ======================== Coordination ========================
var (
	LeasesV1 = schema.GroupVersionResource{
		Group:    "coordination.k8s.io",
		Version:  "v1",
		Resource: "leases",
	}
)
