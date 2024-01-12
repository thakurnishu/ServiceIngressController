package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	appsInformer "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	appsLister "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	workqueue "k8s.io/client-go/util/workqueue"
)

type controller struct {
	clientset               *kubernetes.Clientset
	deploymentLister        appsLister.DeploymentLister
	isDeploymentCacheSynced cache.InformerSynced
	queue                   workqueue.RateLimitingInterface
}

// func to access controller
func newController(clientset *kubernetes.Clientset, deploymentInformer appsInformer.DeploymentInformer) *controller {

	c := &controller{
		clientset:               clientset,
		deploymentLister:        deploymentInformer.Lister(),
		isDeploymentCacheSynced: deploymentInformer.Informer().HasSynced,
		queue:                   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ekspose"),
	}

	deploymentInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerDetailedFuncs{
			AddFunc:    c.handleAdd,
			DeleteFunc: c.handleDelete,
			// UpdateFunc: handleUpdate,
		},
	)

	return c
}

func (c *controller) run(ch <-chan struct{}) {

	fmt.Println("NOTE: Starting Controller...")

	if !cache.WaitForCacheSync(ch, c.isDeploymentCacheSynced) {
		fmt.Printf("ERROR: waiting for cache to be sycned\n")
	}

	go wait.Until(c.worker, 1*time.Second, ch)

	<-ch
}

func (c *controller) worker() {

	for c.processItem() {
	}
}

func (c *controller) processItem() bool {

	item, shutdown := c.queue.Get()
	if shutdown {
		return false
	}

	defer c.queue.Forget(item)

	key, err := cache.MetaNamespaceKeyFunc(item)
	if err != nil {
		fmt.Printf("ERROR: getting key from cache\n%s\n", err.Error())
	}

	nameSpace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		fmt.Printf("ERROR: spliting key to namespace and name\n%s\n", err.Error())
		return false
	}

	// Check Deployment in k8s cluster system namespace
	if systemNamespaces := map[string]bool{
		"kube-node-lease":    true,
		"kube-public":        true,
		"kube-system":        true,
		"local-path-storage": true,
		"ingress-nginx":      true,
	}; systemNamespaces[nameSpace] {
		return true
	}

	err = c.syncDeployment(nameSpace, name)
	if err != nil {
		// re-try

		fmt.Printf("ERROR: syncing deployment\n%s\n", err.Error())
		return false
	}

	return true
}

func (c *controller) syncDeployment(nameSpace, name string) error {

	deployment, err := c.deploymentLister.Deployments(nameSpace).Get(name)
	if err != nil {
		fmt.Printf("ERROR: getting deployment\n%s\n", err.Error())
	}

	ctx := context.Background()

	// Create SVC
	svc, err := c.createService(deployment, ctx)
	if err != nil {
		fmt.Printf("ERROR: creating service\n%s\n", err.Error())
	}

	// Create Ingress
	err = c.createIngress(*svc, ctx)
	if err != nil {
		fmt.Printf("ERROR: creating ingress\n%s\n", err.Error())
	}
	return nil
}

func (c *controller) createIngress(svc corev1.Service, ctx context.Context) error {

	/*
		we have to figure out port where
			our svc is expose
	*/
	deploymentName := strings.TrimSuffix(svc.Name, "-svc")
	path := fmt.Sprintf("/%s", deploymentName)
	pathType := "Prefix"

	ingress := netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName + "-ingress",
			Namespace: svc.Namespace,
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/rewrite-target": "/",
			},
		},
		Spec: netv1.IngressSpec{
			Rules: []netv1.IngressRule{
				{
					IngressRuleValue: netv1.IngressRuleValue{
						HTTP: &netv1.HTTPIngressRuleValue{
							Paths: []netv1.HTTPIngressPath{
								{
									Path:     path,
									PathType: (*netv1.PathType)(&pathType),
									Backend: netv1.IngressBackend{
										Service: &netv1.IngressServiceBackend{
											Name: svc.Name,
											Port: netv1.ServiceBackendPort{
												Number: 80,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	_, err := c.clientset.NetworkingV1().Ingresses(svc.Namespace).Create(ctx, &ingress, metav1.CreateOptions{})
	return err
}

func (c *controller) createService(deployment *appsv1.Deployment, ctx context.Context) (*corev1.Service, error) {

	/*
		we have to figure out port where
			our deployment container is listening
	*/
	podTemplateLabels := deployment.Spec.Template.Labels

	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployment.Name + "-svc",
			Namespace: deployment.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: podTemplateLabels,
			Ports: []corev1.ServicePort{
				{
					Name: "http",
					Port: 80,
				},
			},
		},
	}

	s, err := c.clientset.CoreV1().Services(deployment.Namespace).Create(ctx, &svc, metav1.CreateOptions{})
	return s, err
}

func (c *controller) handleAdd(new interface{}, isInInitialList bool) {

	fmt.Println("Add was Called")
	c.queue.Add(new)
}

func (c *controller) handleDelete(del interface{}) {

	fmt.Println("Delete was Called")
	c.queue.Add(del)
}

/*
func handleUpdate(oldObj interface{}, newObj interface{}) {
	fmt.Println("Update was Called")
}
*/
