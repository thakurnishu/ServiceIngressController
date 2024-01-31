package controller

import (
	"context"
	"fmt"
	"log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func (c *controller) processItemDeploy() bool {

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
    deployment, err := c.clientset.AppsV1().Deployments(nameSpace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		log.Printf("deleting resources %s\n", name)
		return c.deleteResources(nameSpace, name)
	}

	if systemNamespaces := map[string]bool{
		"kube-node-lease":    true,
		"kube-public":        true,
		"kube-system":        true,
		"local-path-storage": true,
		"ingress-nginx":      true,
		"sik-deployment_controller":     true,
	}; systemNamespaces[nameSpace] {
		return true
	}

    deployment.SetLabels(map[string]string{
        "svc": fmt.Sprintf("%s-svc",deployment.Name),
        "ingress": fmt.Sprintf("%s-ingress",deployment.Name),
        "app": deployment.Name,
    })

    deploy, err := c.clientset.AppsV1().Deployments(deployment.Namespace).Update(ctx, deployment, metav1.UpdateOptions{})
    if err != nil {
		log.Fatalf("ERROR: updating deployment labels\n%s\n", err.Error())
    }

    log.Printf("deployment new labels: %v\n", deploy.GetLabels())

	return c.createResources(nameSpace, name)
}


func (c *controller) createResources(nameSpace, name string) bool {
	// Check Deployment in k8s cluster system namespace


	log.Printf("creating resources %s\n", name)
    err := c.syncDeployment(nameSpace, name)
	if err != nil {
		// re-try
		log.Fatalf("ERROR: syncing deployment\n%s\n", err.Error())
		return false
	}

	return true
}

func (c *controller) syncDeployment(nameSpace, name string) error {

	deployment, err := c.deployController.deploymentLister.Deployments(nameSpace).Get(name)
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

	return nil
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

func (c *controller) handleAddDeployment(newObj interface{}, isInInitialList bool) {

	log.Println("Deployment Created")
	c.queue.Add(newObj)
}

func (c *controller) handleDeleteDeployment(delObj interface{}) {

	log.Println("Deployment Deleted")
	c.queue.Add(delObj)
}
