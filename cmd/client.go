package cmd

import (
	"context"
	"github.com/rivo/tview"
	"k8s.io/apimachinery/pkg/api/errors"
	"time"

	"github.com/pterm/pterm"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"path/filepath"
)

// configClient creates a new Kubernetes client
func configClient() {

	if home := homedir.HomeDir(); home != "" && *kubeconfig == "" {
		*kubeconfig = filepath.Join(home, ".kube", "config")
	}

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		pterm.Fatal.Printfln("kubeconfig error while reading %s\nPlease provide a valid kubeconfig file with \"--kubeconfig <file_path>\"", *kubeconfig)
	}
	config.Burst = 100

	// create the client
	client, err = kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
}

// getClusterInfo configures the namespace to use
func getClusterInfo(namespace *string, kubeCtx *string) {

	config, err := clientcmd.LoadFromFile(*kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	*kubeCtx = config.CurrentContext
	if *namespace == "" {
		*namespace = config.Contexts[*kubeCtx].Namespace

		if len(*namespace) == 0 {
			*namespace = "default"
		}
	}

	// check if namespace exists
	_, err = client.CoreV1().Namespaces().Get(context.TODO(), *namespace, metav1.GetOptions{})
	if err != nil {
		pterm.Warning.Printfln("Namespace %s not found", *namespace)
		listNamespaces()
	}
	pterm.Info.Printfln("Using Context %s", pterm.Green(config.CurrentContext))
	pterm.Info.Printfln("Using Namespace %s", pterm.Green(*namespace))
}

// listNamespaces lists all namespaces in the cluster
func listNamespaces() {
	// get namespaces and prompt user to select one
	namespaces, err := client.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	var ns []string
	for _, n := range namespaces.Items {
		ns = append(ns, n.Name)
	}

	// Use PTerm's interactive select feature to present the options to the user and capture their selection
	// The Show() method displays the options and waits for the user's input
	*namespace, _ = pterm.DefaultInteractiveSelect.
		WithOptions(ns).
		WithDefaultText("Select a Namespace").
		Show()
}

func kGetAllPods(ns string) v1.PodList {
	podList, err := client.CoreV1().Pods(ns).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	return *podList
}

// findPodByLabel finds pods by label
func findPodByLabel(label string) v1.PodList {

	pods, err := client.CoreV1().Pods(*namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: label,
	})
	if statusError, isStatus := err.(*errors.StatusError); isStatus {
		pterm.Error.Printf("Error getting pods in namespace %s: %v\n",
			*namespace, statusError.ErrStatus.Message)
	}
	if err != nil {
		panic(err.Error())
	}

	return *pods
}

// getLopOpts returns the Kubernetes option for logs
func getLopOpts() v1.PodLogOptions {
	var logOpts v1.PodLogOptions
	// Since
	if *since != "" {
		// After
		duration, err := time.ParseDuration(*since)
		if err != nil {
			panic(err.Error())
		}
		s := int64(duration.Seconds())
		logOpts.SinceSeconds = &s
	}
	// Tail
	if *tail != -1 {
		logOpts.TailLines = tail
	}
	// Follow
	logOpts.Follow = *follow

	return logOpts
}

func getPodLogsV2(pod v1.Pod, logOpts v1.PodLogOptions) {

	if *initContainer {
		for _, initC := range pod.Spec.InitContainers {
			if _, ok := monitoredPods[pod.Name]; !ok {
				continue
			}
			monitoredPods[pod.Name].GetRoot().AddChild(tview.NewTreeNode(initC.Name))
			//fmt.Printf("Streamed logs for Pod: %s, Init Container: %s\n", pod.Name, initC.Name)

			logFile := createLogFile(pod.Name, initC.Name)

			wg.Add(1)
			go streamLog(pod, initC, logFile, logOpts)
		}
	}
	for _, container := range pod.Spec.Containers {
		if _, ok := monitoredPods[pod.Name]; !ok {
			continue
		}
		monitoredPods[pod.Name].GetRoot().AddChild(tview.NewTreeNode(container.Name))
		//fmt.Printf("Streamed logs for Pod: %s, Container: %s\n", pod.Name, container.Name)

		logFile := createLogFile(pod.Name, container.Name)

		wg.Add(1)
		go streamLog(pod, container, logFile, logOpts)
	}

}
