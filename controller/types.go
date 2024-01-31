package controller

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"


    appsInformer "k8s.io/client-go/informers/apps/v1"
    coreInformer "k8s.io/client-go/informers/core/v1"
    netInformer "k8s.io/client-go/informers/networking/v1"

    appsLister "k8s.io/client-go/listers/apps/v1"
    coreLister "k8s.io/client-go/listers/core/v1"
    netLister "k8s.io/client-go/listers/networking/v1"
	workqueue "k8s.io/client-go/util/workqueue"
)


type InformerToServe struct {
    DeploymentInformer  appsInformer.DeploymentInformer 
    ServiceInfoermer    coreInformer.ServiceInformer
    IngressInformer     netInformer.IngressInformer 
}

type controller struct {
	clientset               *kubernetes.Clientset
	queue                   workqueue.RateLimitingInterface
    deployController    deployment_controller
    serviceController   service_controller
    ingressController   ingress_controller
}

type deployment_controller struct {
	deploymentLister        appsLister.DeploymentLister
	isDeploymentCacheSynced cache.InformerSynced
}

type service_controller struct {
	serviceLister        coreLister.ServiceLister 
	isServiceCacheSynced cache.InformerSynced
}

type ingress_controller struct {
	ingressLister        netLister.IngressLister 
	isIngressCacheSynced cache.InformerSynced
}
