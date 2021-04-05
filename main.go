package main

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	log "github.com/akamensky/go-log"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type arrayFlags []string

func (i *arrayFlags) String() string {
	return "UNIMPLEMENTED"
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

var myFlags arrayFlags

func main() {
	transLabels := make(arrayFlags, 0)
	flag.Var(&transLabels, "label", "A label to transfer from the node to the pod")
	logLevel := flag.String("log-level", "info", "Log level. Can be 'error', 'info', or 'debug'")
	flag.Parse()

	switch strings.ToUpper(*logLevel) {
	case "DEBUG":
		log.SetLevel(log.DEBUG)
	case "INFO":
		log.SetLevel(log.INFO)
	case "ERROR":
		log.SetLevel(log.ERROR)
	default:
		log.Fatalf("Unknown log level %s", *logLevel)
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal(err.Error())
	}
	// creates the clientset
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err.Error())
	}
	factory := informers.NewSharedInformerFactory(clientSet, 5*time.Minute)
	podI := factory.Core().V1().Pods()
	nodeI := factory.Core().V1().Nodes()
	stopCh := make(chan struct{})
	defer close(stopCh)

	go podI.Informer().Run(stopCh)
	go nodeI.Informer().Run(stopCh)

	go func() {
		log.Info("Waiting for informer sync...")
		if !cache.WaitForCacheSync(stopCh, podI.Informer().HasSynced, nodeI.Informer().HasSynced) {
			log.Error("Failed to sync informers...")
			return
		}
		log.Info("Cache synced")

		nodeLister := nodeI.Lister()
		podLister := podI.Lister()

		for {
			select {
			case <-stopCh:
				return
			default:
			}

			start := time.Now()
			nodeLabels := map[string]map[string]string{}
			nodes, err := nodeLister.List(labels.NewSelector())
			if err != nil {
				log.Fatal(err)
			}

			for _, node := range nodes {
				nodeLabels[node.Name] = map[string]string{}
				for _, label := range transLabels {
					nodeLabels[node.Name][label] = node.Labels[label]
				}
			}
			pods, err := podLister.List(labels.NewSelector())
			if err != nil {
				log.Fatal(err)
			}

			// TODO: parallelize.
			for _, pod := range pods {
				err = mergePodLabels(clientSet, pod, nodeLabels[pod.Spec.NodeName])
				if err != nil {
					log.Errorf("Error applying pod labels: %v\n", err)
				}
			}
			log.Debugf("Finished controller loop on %d nodes, %d pods in %s\n", len(nodes), len(pods), time.Since(start))
			time.Sleep(1 * time.Second)
		}
	}()

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM)
	signal.Notify(sigterm, syscall.SIGINT)
	<-sigterm
	log.Info("Received SIGINT or SIGTERM. Shutting down.")
}

func mergePodLabels(clientSet *kubernetes.Clientset, pod *v1.Pod, labels map[string]string) error {
	needsUpdate := false
	for l, val := range labels {
		if pod.Labels[l] != val {
			needsUpdate = true
			break
		}
	}
	if !needsUpdate {
		return nil
	}
	log.Debugf("Adding labels to pod %s/%s\n", pod.Namespace, pod.Name)
	patch, _ := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": labels,
		},
	})
	_, err := clientSet.CoreV1().Pods(pod.Namespace).Patch(context.Background(), pod.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	return err
}
