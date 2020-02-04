package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"log"
	"net/url"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func parseConfig () []rest.Config {
	var tokenBearerBytes []byte
	var kubeConfig rest.Config
	var kubeConfigList []rest.Config
	var connectTimeout time.Duration

	flag.DurationVar(&connectTimeout, "connect-timeout", 2*time.Second, "Connection timeout with SI time suffix.")
	flag.Parse()

	if flag.NArg() < 3 {
		_, _ = fmt.Fprintf(os.Stderr,"Usage: %s BASE64_CA BASE64_TOKEN API_URL1 API_URL2...\n", os.Args[0])
		os.Exit(2)
	}

	kubeConfigCAData, _ := base64.StdEncoding.DecodeString(flag.Arg(0))
	tokenBearerBytes, _ = base64.StdEncoding.DecodeString(flag.Arg(1))
	kubeConfigBearerToken := string(tokenBearerBytes)

	for i := 2; i < flag.NArg(); i++ {
		kubeServer, err := url.Parse(flag.Arg(i))

		if err != nil {
			log.Printf("Invalid URL: %s\n", flag.Arg(i))
		} else {
			kubeConfig = rest.Config{
				Host:        (*kubeServer).Host,
				APIPath:     (*kubeServer).Path,
				BearerToken: kubeConfigBearerToken,
				Timeout:     connectTimeout,
			}
			kubeConfig.CAData = kubeConfigCAData
			kubeConfigList = append(kubeConfigList, kubeConfig)
		}
	}

	if len(kubeConfigList) < 1 {
		log.Fatalln("Not enough server URL.")
	}

	return kubeConfigList
}

type nodeResourceUsage struct {
	maxMemoryAvailable resource.Quantity
	memoryRequest resource.Quantity
	memoryLimit resource.Quantity
	maxPodsAvailable int64
	pods int64
}

func main () {
	var nodeResource nodeResourceUsage
	nodeResources := make(map[string]nodeResourceUsage)
	kubeConfigList := parseConfig()

	for _, kubeConfig := range kubeConfigList {
		client, err := kubernetes.NewForConfig(&kubeConfig)

		if err != nil {
			log.Printf("Invalid configuration: %s", err.Error())
			continue
		}

		pods, err := client.CoreV1().Pods("").List(metav1.ListOptions{})
		if err != nil {
			log.Printf("Unable to obtain pod list: %s", err.Error())
			continue
		}

		nodes, err := client.CoreV1().Nodes().List(metav1.ListOptions{})
		if err != nil {
			log.Printf("Unable to obtain pod list: %s", err.Error())
			continue
		}

		fmt.Printf("There are %d nodes in the cluster.\n", len(nodes.Items))
		for _, node := range nodes.Items {
			nodeResources[node.Name] = nodeResourceUsage{
				maxMemoryAvailable: node.Status.Allocatable.Memory().DeepCopy(),
				maxPodsAvailable:   node.Status.Allocatable.Pods().Value(),
				memoryRequest:      resource.Quantity{},
				memoryLimit:        resource.Quantity{},
				pods:               0,
			}
		}

		fmt.Printf("There are %d pods in the cluster.\n", len(pods.Items))
		for _, pod := range pods.Items {
			nodeResource = nodeResources[pod.Spec.NodeName]
			nodeResource.pods++
			for _, container := range pod.Spec.Containers {
				nodeResource.memoryLimit.Add(*container.Resources.Limits.Memory())
				nodeResource.memoryRequest.Add(*container.Resources.Requests.Memory())
			}
			nodeResources[pod.Spec.NodeName] = nodeResource
		}

		for key, node := range nodeResources {
			fmt.Printf("%s: Memory: %dMi request (%d%%), %dMi limit (%d%%), %dMi total available. Pods: %d/%d pods (%d%%)\n",
				key,
				node.memoryRequest.ScaledValue(resource.Mega),
				node.memoryRequest.ScaledValue(resource.Mega)*100 / node.maxMemoryAvailable.ScaledValue(resource.Mega),
				node.memoryLimit.ScaledValue(resource.Mega),
				node.memoryLimit.ScaledValue(resource.Mega)*100 / node.maxMemoryAvailable.ScaledValue(resource.Mega),
				node.maxMemoryAvailable.ScaledValue(resource.Mega),
				node.pods,
				node.maxPodsAvailable,
				node.pods*100/node.maxPodsAvailable,
			)
		}

		os.Exit(0)
	}

	log.Fatalf("Could not connect to cluster.\n")
}