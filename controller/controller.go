package controller

import (
	"log"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	workqueue "k8s.io/client-go/util/workqueue"
)

func NewController(client *kubernetes.Clientset, allInformer InformerToServe) *controller {

    c := &controller{
        clientset: client,
        queue:  workqueue.NewNamedRateLimitingQueue(
            workqueue.DefaultControllerRateLimiter(),
            "sik",
        ),
        deployController: deployment_controller{
            deploymentLister: allInformer.DeploymentInformer.Lister(),
            isDeploymentCacheSynced: allInformer.DeploymentInformer.Informer().HasSynced,
        },
        serviceController: service_controller{
            serviceLister: allInformer.ServiceInfoermer.Lister(),
            isServiceCacheSynced: allInformer.ServiceInfoermer.Informer().HasSynced,
        },
        ingressController: ingress_controller{
            ingressLister: allInformer.IngressInformer.Lister(),
            isIngressCacheSynced: allInformer.IngressInformer.Informer().HasSynced,
        },
    }
    
    allInformer.DeploymentInformer.Informer().AddEventHandler(
        cache.ResourceEventHandlerDetailedFuncs{
           AddFunc: c.handleAddDeployment, 
           DeleteFunc: c.handleDeleteDeployment,
        },
    )
    allInformer.ServiceInfoermer.Informer().AddEventHandler(
        cache.ResourceEventHandlerDetailedFuncs{
            DeleteFunc: c.handleDeleteService,
        },
    )
    allInformer.IngressInformer.Informer().AddEventHandler(
        cache.ResourceEventHandlerDetailedFuncs{
            DeleteFunc: c.handleDeleteIngress,
        },
    )

    return c
}  

func (c *controller) run(deployCh <-chan struct{}, serviceCh<-chan struct{}, ingressCh <-chan struct{}) {

	log.Println("Starting Controller...")

    waitForDeploymentSync := cache.WaitForCacheSync(deployCh, c.deployController.isDeploymentCacheSynced)
    waitForServiceSync := cache.WaitForCacheSync(serviceCh, c.serviceController.isServiceCacheSynced)
    waitForIngressSync := cache.WaitForCacheSync(ingressCh, c.ingressController.isIngressCacheSynced)

    if !(waitForDeploymentSync && waitForServiceSync && waitForIngressSync) {
		log.Printf("waiting for cache to be sycned\n")
    }

	go wait.Until(func() {c.deployWorker()}, 1*time.Second, deployCh)
	go wait.Until(func() {c.serviceWorker()}, 1*time.Second, deployCh)
	go wait.Until(func() {c.ingressWorker()}, 1*time.Second, deployCh)

	<-deployCh
    <-serviceCh
    <-ingressCh
} 

func (c *controller) deployWorker() {
	for c.processItemDeploy() {
	}
}

func (c *controller) serviceWorker() {
	for c.processItemService() {
	}
}

func (c *controller) ingressWorker() {
	for c.processItemIngress() {
	}
}
