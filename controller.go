package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/fatih/color"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

	color.Set(color.FgHiGreen)
	log.Println("Starting Controller...")
	color.Unset()

	// if false fail
	if !cache.WaitForCacheSync(ch, c.isDeploymentCacheSynced) {
		color.Set(color.FgHiRed)
		log.Fatalf("ERROR: waiting for cache to be sycned\n")
		color.Unset()
	}

	go wait.Until(c.worker, 1*time.Second, ch)

	<-ch
}

func (c *controller) worker() {

	for c.processItem() {
	}
}

func (c *controller) processItem() bool {

	color.Set(color.FgHiRed)
	item, shutdown := c.queue.Get()
	if shutdown {
		return false
	}

	defer c.queue.Forget(item)

	key, err := cache.MetaNamespaceKeyFunc(item)
	if err != nil {
		log.Fatalf("ERROR: getting key from cache\n%s\n", err.Error())
	}

	nameSpace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		log.Fatalf("ERROR: spliting key to namespace and name\n%s\n", err.Error())
		return false
	}

	ctx := context.Background()
	// Check if object is still in cluster (added/deleted)
	_, err = c.clientset.AppsV1().Deployments(nameSpace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		log.Printf("deleting resources %s\n", name)
		return c.deleteResources(nameSpace, name)
	}
	return c.createResources(nameSpace, name)

}

func (c *controller) createResources(nameSpace, name string) bool {
	// Check Deployment in k8s cluster system namespace
	if systemNamespaces := map[string]bool{
		"kube-node-lease":    true,
		"kube-public":        true,
		"kube-system":        true,
		"local-path-storage": true,
		"ingress-nginx":      true,
		"sik-controller":     true,
	}; systemNamespaces[nameSpace] {
		return true
	}

	log.Printf("creating resources %s\n", name)
	err := c.syncDeployment(nameSpace, name)
	if err != nil {
		// re-try
		log.Fatalf("ERROR: syncing deployment\n%s\n", err.Error())
		return false
	}

	color.Unset()
	return true
}

func (c *controller) deleteResources(nameSpace, name string) bool {

	ctx := context.Background()
	// delete service
	err := c.clientset.CoreV1().Services(nameSpace).Delete(ctx, name+"-svc", metav1.DeleteOptions{})
	if err != nil {
		log.Fatalf("failed to delete service: %s-svc", name)
	}

	// delete ingress
	err = c.clientset.NetworkingV1().Ingresses(nameSpace).Delete(ctx, name+"-ingress", metav1.DeleteOptions{})
	if err != nil {
		log.Fatalf("failed to delete ingress: %s-ingress", name)
	}

	return true
}

func (c *controller) syncDeployment(nameSpace, name string) error {

	color.Set(color.FgHiRed)
	deployment, err := c.deploymentLister.Deployments(nameSpace).Get(name)
	if err != nil {
		log.Fatalf("ERROR: getting deployment\n%s\n", err.Error())
	}

	ctx := context.Background()

	// Create SVC
	svc, err := c.createService(deployment, ctx)
	if err != nil {
		log.Fatalf("ERROR: creating service\n%s\n", err.Error())
	}

	// Create Ingress
	err = c.createIngress(*svc, ctx)
	if err != nil {
		log.Fatalf("ERROR: creating ingress\n%s\n", err.Error())
	}
	color.Unset()
	return nil
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

func (c *controller) createIngress(svc corev1.Service, ctx context.Context) error {

	/*
		we have to figure out port where
			our svc is expose
	*/
	deploymentName := strings.TrimSuffix(svc.Name, "-svc")
	path := fmt.Sprintf("/%s", svc.Name)
	pathType := "Prefix"
	ingressClassName := "nginx"

	ingress := netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName + "-ingress",
			Namespace: svc.Namespace,
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/rewrite-target": "/",
			},
		},
		Spec: netv1.IngressSpec{
			IngressClassName: &ingressClassName,
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

func (c *controller) handleAdd(new interface{}, isInInitialList bool) {

	log.Println("Added")
	c.queue.Add(new)
}

func (c *controller) handleDelete(del interface{}) {

	log.Println("Deleted")
	c.queue.Add(del)
}

/*
func handleUpdate(oldObj interface{}, newObj interface{}) {
	fmt.Println("Update was Called")
}
*/
