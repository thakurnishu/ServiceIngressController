package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func KubeconfigHome() string {

	// To get kubeconfig file location
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("ERROR getting UserHome dir: \n%v\n", err.Error())
	}

	kubeconfigPath := filepath.Join(homeDir, ".kube", "config")

	return kubeconfigPath
}

func GetTypedClientSet(kubeconfig *string) *kubernetes.Clientset {

	var config *rest.Config

	if _, err := os.Stat(*kubeconfig); err == nil {

		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			fmt.Printf("ERROR Building Config from Flag: \n%v\n", err.Error())
		}
	} else {

		config, err = rest.InClusterConfig()
		if err != nil {
			fmt.Printf("ERROR getting Config from K8S Cluster: \n%v\n", err.Error())
		}
	}

	// Typed ClientSet
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("ERROR Creating Typed ClientSet from Config: \n%v\n", err.Error())
	}

	return clientSet
}

func main() {

	kubeconfigHome := KubeconfigHome()
	// File location from user ( --kubeconfig ) or homeDir/.kube/config
	kubeconfig := flag.String("kubeconfig", kubeconfigHome, "Location of KubeConfig file")

	// Parse flags once
	flag.Parse()

	clientset := GetTypedClientSet(kubeconfig)

	// Shared Informer Factory
	sharedInformerFactory := informers.NewSharedInformerFactory(clientset, 10*time.Minute)

	// Deployment Informer
	deploymentInformer := sharedInformerFactory.Apps().V1().Deployments()

	ch := make(chan struct{})

	c := newController(clientset, deploymentInformer)

	sharedInformerFactory.Start(ch)
	c.run(ch)
}
