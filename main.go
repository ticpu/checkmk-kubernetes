package main

import (
	"encoding/base64"
	"flag"
	"fmt"
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
				Timeout:     2 * time.Second,
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

func main () {
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

		fmt.Printf("There are %d pods in the cluster.\n", len(pods.Items))
		os.Exit(0)
	}

	log.Fatalf("Could not connect to cluster.\n")
}