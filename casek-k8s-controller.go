package main

import (
	"flag"
	"fmt"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"time"
)

func main() {
	kubeconfig := ""
	flag.StringVar(&kubeconfig, "kubeconfig", kubeconfig, "Path to kubecofig")
	flag.Parse()
	if kubeconfig == "" {
		kubeconfig = os.Getenv("KUBECONFIG")
	}

	var (
		config *rest.Config
		err    error
	)

	if kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error creating client %v", err)
		os.Exit(1)
	}
	client := kubernetes.NewForConfigOrDie(config)
	sharedInformers := informers.NewSharedInformerFactory(client, 10*time.Minute)
	casekController := NewCasekController(client,sharedInformers.Core().V1().Secrets())

	sharedInformers.Start(nil)
	casekController.Run(nil)
}
