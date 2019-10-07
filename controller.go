package main

import (
	"fmt"
	v14 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v15 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	v13 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	v12 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"log"
)

const (
	secretSyncType        = "casek14/secretsync"
	secretSourceNamespace = "secretsync"
)

var namespaceBlacklist = map[string]bool{
	"kube-public":         true,
	"kube-system":         true,
	secretSourceNamespace: true,
}

type CasekController struct {
	secretGetter          v1.SecretsGetter
	secretLister          v12.SecretLister
	secretListerSynced    cache.InformerSynced
	namespaceGetter       v1.NamespacesGetter
	namespaceLister       v12.NamespaceLister
	namespaceListerSynced cache.InformerSynced
}

func NewCasekController(client *kubernetes.Clientset, secretInformer v13.SecretInformer,
	namespaceInformer v13.NamespaceInformer) *CasekController {
	c := &CasekController{
		secretGetter:          client.CoreV1(),
		secretLister:          secretInformer.Lister(),
		secretListerSynced:    secretInformer.Informer().HasSynced,
		namespaceGetter:       client.CoreV1(),
		namespaceLister:       namespaceInformer.Lister(),
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
	//log.Print("Waiting for stop signal")
	//<-stop
	//log.Print("Received stop signal")
	c.doSync()
}

func (c *CasekController) onAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		log.Printf("[ERR] - onAdd: error getting key for %#v: %v", obj, err)
		runtime.HandleError(err)
	}
	log.Printf("onAdd: %v", key)
	//c.handleSecretChange(obj)
}

func (c *CasekController) onUpdate(oldObj, newObj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(oldObj)
	if err != nil {
		log.Printf("[ERR] - onUpdate: error getting key for %#v: %v", oldObj, err)
		runtime.HandleError(err)
	}
	log.Printf("onUpdate: %v", key)
	//c.handleSecretChange(newObj)
}

func (c *CasekController) onDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		log.Printf("[ERR] - onDelete: error deleting key for %#v: %v", obj, err)
		runtime.HandleError(err)
	}
	log.Printf("onDelete: %v", key)
	//c.handleSecretChange(obj)
}

func (c *CasekController) handleSecretChange(obj interface{}) {
	secret, ok := obj.(*v14.Secret)
	if !ok {
		//TODO:
		return
	}

	if secret.ObjectMeta.Namespace != secretSourceNamespace {
		log.Printf("Skipping namespace in wrong secret: %v", secret.Namespace)
		return
	}

	if secret.Type != secretSyncType {
		log.Printf("Skipping secret of wrong type: %v", secret.Name)
		return
	}

	log.Printf("Do something with this secret %v", secret.Name)
	nss, err := c.namespaceGetter.Namespaces().List(v15.ListOptions{})
	if err != nil {
		log.Printf("Error listing namespaces: %v", err)
		return
	}

	for _, ns := range nss.Items {
		nsName := ns.ObjectMeta.Name
		if _, ok := namespaceBlacklist[nsName]; ok {
			log.Printf("Skipping namespace on blacklist: %v", nsName)
		}
		log.Printf("We should copy %s to namespace %s", secret.ObjectMeta.Name, ns.ObjectMeta.Name)

		//c.copySecretToNamespace(secret, nsName)
	}

}

func (c *CasekController) SyncNamespace(secrets []*v14.Secret, nsName string) {
	// 1.
	for _, secret := range secrets {
		newSecret := secret.DeepCopy()
		newSecret.Namespace = nsName
		newSecret.ResourceVersion = ""
		newSecret.UID = ""
		_, err := c.secretGetter.Secrets(nsName).Create(newSecret)

		if errors.IsAlreadyExists(err) {
			_, err = c.secretGetter.Secrets(nsName).Update(newSecret)
		}
		if err != nil {
			log.Printf("NEW-SECRET [ERR]: %v", err)

		}
	}

	// 2.
	srcSecrets := sets.String{}
	targetSecrets := sets.String{}

	for _, secret := range secrets{
		srcSecrets.Insert(secret.Name)
	}

	targetSecretList, err := c.getSecretInNs(nsName)
	if err != nil {
		log.Printf("%v",err)
	}

	for _, secret := range targetSecretList{
		targetSecrets.Insert(secret.Name)
	}


	deleteSet := targetSecrets.Difference(srcSecrets)
	for secretName, _ := range deleteSet{
		log.Printf("DELETE: %v", secretName)
		_ = c.secretGetter.Secrets(nsName).Delete(secretName,nil)
	}
}

func (c *CasekController) doSync() {
	log.Printf("HERE!")
	srcSecrets, err := c.getSecretInNs(secretSourceNamespace)

	rawNamespaces, err := c.namespaceLister.List(labels.Everything())
	if err != nil {
		fmt.Printf("Oh no %v", err)
	}
	var targetedNamespaces []*v14.Namespace
	for _, ns := range rawNamespaces {
		if _, ok := ns.Annotations[secretSyncType]; ok {
			targetedNamespaces = append(targetedNamespaces, ns)
		}
	}

	for _, ns := range targetedNamespaces {
		c.SyncNamespace(srcSecrets, ns.Name)
	}
}

func (c *CasekController) getSecretInNs(ns string) ([]*v14.Secret, error) {
	rawSecret, err := c.secretLister.Secrets(ns).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	var secrets []*v14.Secret
	for _, secret := range rawSecret {
		if _, ok := secret.Annotations[secretSyncType]; ok {
			secrets = append(secrets, secret)
		}
	}

	return secrets, nil

}
