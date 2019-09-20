package main

import (
	"k8s.io/apimachinery/pkg/util/runtime"
	v13 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	v12 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"log"
)

type CasekController struct {
	secretGetter       v1.SecretsGetter
	secretLister       v12.SecretLister
	secretListerSynced cache.InformerSynced
}

func NewCasekController(client *kubernetes.Clientset, secretInformer v13.SecretInformer) *CasekController {
	c := &CasekController{
		secretGetter:       client.CoreV1(),
		secretLister:       secretInformer.Lister(),
		secretListerSynced: secretInformer.Informer().HasSynced,
	}
	secretInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				c.onAdd(obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				c.onUpdate(oldObj, newObj)
			},
			DeleteFunc: func(obj interface{}) {
				c.onDelete(obj)
			},
		})

	return c
}

func (c *CasekController) Run(stop <-chan struct{}) {
	log.Print("Waiting for cache sync")
	if !cache.WaitForCacheSync(stop, c.secretListerSynced) {
		log.Print("Timout waiting for cache sync")
		return
	}

	log.Print("Caches are synced")
	log.Print("Waiting for stop signal")
	<-stop
	log.Print("Received stop signal")
}

func (c *CasekController) onAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		log.Printf("[ERR] - onAdd: error getting key for %#v: %v", obj, err)
		runtime.HandleError(err)
	}
	log.Printf("onAdd: %v", key)

}

func (c *CasekController) onUpdate(oldObj, _ interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(oldObj)
	if err != nil {
		log.Printf("[ERR] - onUpdate: error getting key for %#v: %v", oldObj, err)
		runtime.HandleError(err)
	}
	log.Printf("onUpdate: %v", key)
}

func (c *CasekController) onDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		log.Printf("[ERR] - onDelete: error deleting key for %#v: %v", obj, err)
		runtime.HandleError(err)
	}
	log.Printf("onDelete: %v", key)
}
