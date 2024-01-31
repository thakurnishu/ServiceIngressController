package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fatih/color"
	"github.com/thakurnishu/service-ingress-k8s-controller/controller"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
  
    appsInformer "k8s.io/client-go/informers/apps/v1"
    coreInformer "k8s.io/client-go/informers/core/v1"
    netInformer "k8s.io/client-go/informers/networking/v1"
)    

func KubeconfigHome() string {

	// To get kubeconfig file location
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Printf("ERROR getting UserHome dir \n%v\n", err.Error())
	}

	kubeconfigPath := filepath.Join(homeDir, ".kube", "config")

	return kubeconfigPath
}

func GetTypedClientSet(kubeconfig *string) *kubernetes.Clientset {

	var config *rest.Config

	if _, err := os.Stat(*kubeconfig); err == nil {

		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			log.Fatalf("[ERROR Building Config from Flag] \n%v\n", err.Error())
		}
	} else {

		config, err = rest.InClusterConfig()
		if err != nil {
			log.Fatalf("ERROR getting Config from K8S Cluster \n%v\n", err.Error())
		}
	}

	// Typed ClientSet
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("ERROR Creating Typed ClientSet from Config \n%v\n", err.Error())
	}

	return clientSet
}


func main() {

	color.Set(color.FgHiRed)
	kubeconfigHome := KubeconfigHome()
	// File location from user ( --kubeconfig ) or homeDir/.kube/config
	kubeconfig := flag.String("kubeconfig", kubeconfigHome, "Location of KubeConfig file")

	// Parse flags once
	flag.Parse()

	clientset := GetTypedClientSet(kubeconfig)

	// Shared Informer Factory
	sharedInformerFactory := informers.NewSharedInformerFactory(clientset, 10*time.Minute)

	deploymentInformer  := sharedInformerFactory.Apps().V1().Deployments()
	serviceInformer    := sharedInformerFactory.Core().V1().Services()
	ingressInformer     := sharedInformerFactory.Networking().V1().Ingresses()

	// Deployment Informer
    allInformer := &controller.InformerToServe{
        DeploymentInformer: deploymentInformer,
        ServiceInfoermer: serviceInformer,
        IngressInformer: ingressInformer,
    }

	color.Unset()
	deployCh := make(chan struct{})
	serviceCh := make(chan struct{})
	ingressCh := make(chan struct{})

	c := controller.NewController(clientset, *allInformer)

	sharedInformerFactory.Start(ch)
	c.Run(deployCh, serviceCh, ingressCh)
}
