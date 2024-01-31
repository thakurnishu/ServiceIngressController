package controller

import (
	"context"
	"log"
	"strings"

	"k8s.io/client-go/tools/cache"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *controller) processItemService() bool {
    item, shutdown := c.queue.Get()
    if shutdown {
        return false
    }
    defer c.queue.Forget(item) 

    key, err := cache.MetaNamespaceKeyFunc(item)
    if err != nil {
        log.Fatalf("ERROR: getting namespace key\n",err.Error())
    }
    
    namespace, name, err := cache.SplitMetaNamespaceKey(key)
    if err != nil {
        log.Fatalf("ERROR: getting namespace and name %s\n",err.Error())
    }

    ctx := context.Background()
    deployName := strings.Trim(name, "-svc")
    // Check if owner of this service still exist
    deploy, err := c.clientset.AppsV1().Deployments(namespace).Get(ctx, deployName, metav1.GetOptions{})
    if err != nil {
        if apierrors.IsNotFound(err) {
            return true
        } 
        log.Fatalf("ERROR: getting deployment (service) %s\n", err.Error())
        return true
    }

    // and if owner still exist recreate this service
    _, err = c.createService(deploy, ctx)
    if err != nil {
        log.Fatalf("ERROR: creating service (recreate) %s\n", err.Error())
    }

    return true
}

func (c *controller) handleDeleteService(delObj interface{}) {

	log.Println("Deployment Deleted")
	c.queue.Add(delObj)
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
            OwnerReferences: []metav1.OwnerReference{
                {
                    Name: deployment.Name,
                    APIVersion: deployment.APIVersion,
                    Kind: deployment.Kind,
                },
            },
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
