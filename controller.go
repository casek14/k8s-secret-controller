package main

import (
	v14 "k8s.io/api/core/v1"
	v15 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	v13 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	v12 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"log"
)

const (
	secretSyncType = "casek14/secretsync"
	secretSourceNamespace = "secretsync"
)

type CasekController struct {
	secretGetter       v1.SecretsGetter
	secretLister       v12.SecretLister
	secretListerSynced cache.InformerSynced
	namespaceGetter       v1.NamespacesGetter
	namespaceLister       v12.NamespaceLister
	namespaceListerSynced cache.InformerSynced
}

func NewCasekController(client *kubernetes.Clientset, secretInformer v13.SecretInformer,
						namespaceInformer v13.NamespaceInformer) *CasekController {
	c := &CasekController{
		secretGetter:       client.CoreV1(),
		secretLister:       secretInformer.Lister(),
		secretListerSynced: secretInformer.Informer().HasSynced,
		namespaceGetter: client.CoreV1(),
		namespaceLister: namespaceInformer.Lister(),
		namespaceListerSynced: namespaceInformer.Informer().HasSynced,
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
	if !cache.WaitForCacheSync(stop, c.secretListerSynced, c.namespaceListerSynced) {
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
	c.handleSecretChange(obj)
}

func (c *CasekController) onUpdate(oldObj, newObj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(oldObj)
	if err != nil {
		log.Printf("[ERR] - onUpdate: error getting key for %#v: %v", oldObj, err)
		runtime.HandleError(err)
	}
	log.Printf("onUpdate: %v", key)
	c.handleSecretChange(newObj)
}

func (c *CasekController) onDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		log.Printf("[ERR] - onDelete: error deleting key for %#v: %v", obj, err)
		runtime.HandleError(err)
	}
	log.Printf("onDelete: %v", key)
	c.handleSecretChange(obj)
}

func(c *CasekController) handleSecretChange(obj interface{}){
	secret, ok := obj.(*v14.Secret)
	if !ok {
		//TODO:
		return
	}

	if secret.ObjectMeta.Namespace != secretSourceNamespace {
		log.Printf("Skipping namespace in wrong secret: %v",secret.Namespace)
		return
	}

	if secret.Type != secretSyncType {
		log.Printf("Skipping secret of wrong type: %v",secret.Name)
		return
	}

	log.Printf("Do something with this secret %v", secret.Name)
	nss, err := c.namespaceGetter.Namespaces().List(v15.ListOptions{})
	if err != nil{
		log.Printf("Error listing namespaces: %v", err)
		return
	}

	for _, ns := range nss.Items{
		log.Printf("We should copy %s to namespace %s", secret.ObjectMeta.Name, ns.ObjectMeta.Name)
	}


}
