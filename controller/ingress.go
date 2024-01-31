package controller

import (
	"context"
	"fmt"
	"log"
	"strings"

	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *controller) processItemIngress() bool {
    return true
}

func (c *controller) handleDeleteIngress(delObj interface{}) {

	log.Println("Service Deleted")
	c.queue.Add(delObj)
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
            OwnerReferences: []metav1.OwnerReference{
                {
                    Name: deploymentName,
                    Kind: "Deployment",
                    APIVersion: "apps/v1",
                },
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
