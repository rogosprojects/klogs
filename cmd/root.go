/*
Package cmd is the entry point for the command line tool. It defines the root command and its flags.
*/
package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mattn/go-tty"

	"atomicgo.dev/keyboard/keys"
	"github.com/pterm/pterm"
	"github.com/pterm/pterm/putils"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

var (
	// BuildVersion is the version of the build, passed in by the build system
	BuildVersion = "development"
)

// Flags input
var (
	kubeconfig, namespace, since, logPath *string
	client                                *kubernetes.Clientset
	labels                                *[]string
	tail                                  *int64
	follow                                *bool
	printVersion                          *bool
)

var (
	allPods        *bool
	anyLogFound    bool
	defaultLogPath = "logs/" + time.Now().Format("2006-01-02T15-04")
)

// NotifyReadSize notify some bytes have been read
type NotifyReadSize func(total int, delta int)

// MeteredReader decorate Reader to measure reads
type MeteredReader struct {
	reader io.Reader
	total  int
	notify NotifyReadSize
}

// notify progress through specified function
func (w *MeteredReader) Read(p []byte) (int, error) {
	size, err := w.reader.Read(p)
	w.total += size
	w.notify(w.total, size)
	return size, err
}

// splashScreen prints the splash screen!
func splashScreen() {

	err := pterm.DefaultBigText.WithLetters(
		putils.LettersFromStringWithStyle("K", pterm.FgBlue.ToStyle()),
		putils.LettersFromStringWithStyle("Logs", pterm.FgWhite.ToStyle())).
		Render()
	if err != nil {
		return
	} // Render the big text to the terminal

}

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

	// create the client
	client, err = kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
}

// configNamespace configures the namespace to use
func configNamespace() {
	if *namespace == "" {
		*namespace = getCurrentNamespace(*kubeconfig)
	}

	// check if namespace exists
	_, err := client.CoreV1().Namespaces().Get(context.TODO(), *namespace, metav1.GetOptions{})
	if err != nil {
		pterm.Warning.Printfln("Namespace %s not found", *namespace)
		listNamespaces()
	}

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

// listAllPods lists all the ready pods in the namespace
func listAllPods() v1.PodList {
	var _podList v1.PodList
	pods, err := client.CoreV1().Pods(*namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	var podMap = make(map[string]v1.Pod)
	var podNames []string
	for _, pod := range pods.Items {
		// is the pod ready?
		for _, condition := range pod.Status.Conditions {
			if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
				podMap[pod.Name] = pod
				podNames = append(podNames, pod.Name)
				break
			}
		}
	}

	if len(podNames) == 0 {
		pterm.Error.Printfln("No pods found in namespace %s", *namespace)
		return _podList
	}

	if !*allPods {
		podNames = showInteractivePodSelect(podNames)
		if len(podNames) == 0 {
			pterm.Error.Printfln("No pods selected")
			return _podList
		}
	}

	// collect info only for the selected pods
	for _, podName := range podNames {
		_podList.Items = append(_podList.Items, podMap[podName])
	}
	return _podList
}

// showInteractivePodSelect shows an interactive multiselect printer with the pod names
func showInteractivePodSelect(podNames []string) []string {
	// Create a new interactive multiselect printer with the options
	// Disable the filter and set the keys for confirming and selecting options
	printer := pterm.DefaultInteractiveMultiselect.
		WithOptions(podNames).
		WithFilter(false).
		WithKeyConfirm(keys.Enter).
		WithKeySelect(keys.Space).
		WithMaxHeight(15).
		WithDefaultText("Select Pods to get logs")

	// Show the interactive multiselect and get the selected options
	selectedPods, _ := printer.Show()

	return selectedPods
}

// Get the default namespace specified in the KUBECONFIG file current context
func getCurrentNamespace(kubeconfig string) string {

	config, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		panic(err.Error())
	}
	ns := config.Contexts[config.CurrentContext].Namespace

	if len(ns) == 0 {
		ns = "default"
	}

	return ns
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

// getPodLogs gets logs for the pods
func getPodLogs(pods v1.PodList, logOpts v1.PodLogOptions) {
	var wg sync.WaitGroup
	// Create a multi printer for managing multiple printers
	multiPrinter := pterm.DefaultMultiPrinter
	multiPrinter.Start()

	for _, pod := range pods.Items {
		var _podTree = pterm.TreeNode{
			Text: pterm.Info.
				WithPrefix(pterm.Prefix{Text: "[Pod]", Style: pterm.Info.MessageStyle}).
				WithMessageStyle(pterm.DefaultBasicText.Style).
				Sprintf(pod.Name),
		}
		var containerTree []pterm.TreeNode

		for _, container := range pod.Spec.Containers {
			containerTree = append(containerTree, pterm.TreeNode{Text: container.Name})
			_podTree.Children = containerTree

			wg.Add(1)
			go streamLog(pod, container, logOpts, &wg, &multiPrinter)
		}
		err := pterm.DefaultTree.WithRoot(_podTree).Render()
		if err != nil {
			return
		}
	}
	if *follow {
		pterm.Info.Printfln("Press %s to stop streaming logs in %s", pterm.Green("q"), pterm.Green(*logPath))
		pressKeyToExit()
	}

	// wait for all goroutines to finish
	wg.Wait()
}

// streamLog streams logs for the container
func streamLog(pod v1.Pod, container v1.Container, logOpts v1.PodLogOptions, wg *sync.WaitGroup, multiPrinter *pterm.MultiPrinter) {
	defer wg.Done()

	logOpts.Container = container.Name
	// get logs for the container
	req := client.CoreV1().Pods(*namespace).GetLogs(pod.Name, &logOpts)

	// get logs
	logs, err := req.Stream(context.Background())
	if err != nil {
		pterm.Error.Printfln("Error getting logs for container %s\n%v", container.Name, err)
		//containerTree = append(containerTree, pterm.TreeNode{Text: pterm.Red(container.Name)})
		return
	}

	writeLogToDisk(logs, pod.Name, container.Name, multiPrinter)

}

// findPodByLabel finds pods by label
func findPodByLabel(label string) v1.PodList {
	pterm.Info.Printf("Getting Pods in namespace %s with label %s\n\n", pterm.Green(*namespace), pterm.Green(label))

	pods, err := client.CoreV1().Pods(*namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: label,
	})
	if statusError, isStatus := err.(*errors.StatusError); isStatus {
		fmt.Printf("Error getting pods in namespace %s: %v\n",
			*namespace, statusError.ErrStatus.Message)
	}
	if err != nil {
		panic(err.Error())
	}

	// if pods are not found print message
	if len(pods.Items) == 0 {
		pterm.Error.Printfln("No pods found in namespace %s with label %s\n", *namespace, label)
	}

	return *pods
}

// writeLogToDisk writes logs to disk
func writeLogToDisk(logs io.ReadCloser, podName string, containerName string, multiPrinter *pterm.MultiPrinter) {
	anyLogFound = true

	logName := fmt.Sprintf("%s-%s.log", podName, containerName)

	defer func(logs io.ReadCloser) {
		err := logs.Close()
		if err != nil {
			panic(err.Error())
		}
	}(logs)

	// Create the log file
	if err := os.MkdirAll(*logPath, 0755); err != nil {
		panic(err.Error())
	}
	logFilePath := filepath.Join(*logPath, logName)
	logFile, err := os.Create(logFilePath)

	if err != nil {
		panic(err.Error())
	}
	defer func(logFile *os.File) {
		err := logFile.Close()
		if err != nil {
			panic(err.Error())
		}
	}(logFile)

	spinnerLog, _ := pterm.DefaultSpinner.WithWriter(multiPrinter.NewWriter()).
		WithRemoveWhenDone(false).Start("Acquiring logs...")
	defer spinnerLog.Stop()

	reader := &MeteredReader{reader: bufio.NewReader(logs), notify: func(total, delta int) {
		s := pterm.Style{pterm.FgWhite, pterm.BgDefault, pterm.Bold, pterm.Italic}

		spinnerLog.UpdateText(pterm.Info.WithPrefix(
			pterm.Prefix{
				Text:  convertBytes(total),
				Style: &s,
			}).
			WithMessageStyle(&s).
			Sprintf("%s/%s", podName, containerName))

	}}

	// Create a buffered reader and writer
	writer := bufio.NewWriter(logFile)

	// Copy data from the reader to the writer
	if _, err := io.Copy(writer, reader); err != nil {
		panic(err.Error())
	}

	// Flush any remaining data to the file
	if err := writer.Flush(); err != nil {
		panic(err.Error())
	}
}

func convertBytes(bytes int) string {
	if bytes == 0 {
		return pterm.Red(" (0 B)")
	}
	if bytes < 1024 {
		return pterm.Sprintf(" (%d B)", bytes)
	}
	if bytes < 1024*1024 {
		return pterm.Sprintf(" (%d KB)", bytes/1024)
	}
	return pterm.Sprintf(" (%d MB)", bytes/1024/1024)
}

var rootCmd = &cobra.Command{
	Use:   "klogs",
	Short: "Get logs from Pods, super fast! 🚀",
	Long: `klogs is a CLI tool to get logs from Kubernetes Pods.
It is designed to be fast and efficient, and can get logs from multiple Pods/Containers at once. Blazing fast. 🔥`,

	Run: func(cmd *cobra.Command, args []string) {
		var podList v1.PodList

		if *printVersion {
			pterm.Info.Printfln("Version: %s", BuildVersion)
			os.Exit(0)
		}

		splashScreen()

		configClient()
		configNamespace()

		if len(*labels) == 0 {
			podList = listAllPods()
		} else {
			for _, l := range *labels {
				podList.Items = append(podList.Items, findPodByLabel(l).Items...)
			}
		}

		getPodLogs(podList, getLopOpts())

		if anyLogFound {
			pterm.Info.Printfln("Logs saved to %s", pterm.Green(*logPath))
		}
	},
}

func pressKeyToExit() {
	t, errTty := tty.Open()
	if errTty != nil {
		panic(errTty)
	}

	go func() {
		defer t.Close()
		for {
			key, err := t.ReadRune()
			if err != nil {
				panic(err)
			}

			// if pressed q or Q
			if key == 113 || key == 81 {
				pterm.Info.Printfln("Exiting")
				break
			}
		}
		os.Exit(0)
	}()

}

// Execute is the entry point for the command
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	namespace = rootCmd.Flags().StringP("namespace", "n", "", "Select namespace")
	labels = rootCmd.Flags().StringArrayP("label", "l", []string{}, "Select label")
	logPath = rootCmd.Flags().StringP("logpath", "p", defaultLogPath, "Custom log path")
	kubeconfig = rootCmd.Flags().String("kubeconfig", "", "(optional) Absolute path to the kubeconfig file")
	allPods = rootCmd.Flags().BoolP("all", "a", false, "Get logs for all pods in the namespace")
	since = rootCmd.Flags().StringP("since", "s", "", "Only return logs newer than a relative duration like 5s, 2m, or 3h. Defaults to all logs.")
	tail = rootCmd.Flags().Int64P("tail", "t", -1, "Lines of the most recent log to save")
	follow = rootCmd.Flags().BoolP("follow", "f", false, "Specify if the logs should be streamed")
	printVersion = rootCmd.Flags().BoolP("version", "v", false, "Print the version of the tool")

}
