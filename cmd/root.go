/*
Package cmd is the entry point for the command line tool. It defines the root command and its flags.
*/
package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

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
var (
	kubeconfig, namespace, customLogPath *string
	client                               *kubernetes.Clientset
	labels                               *[]string
)

var (
	fileLogs    = fileLog{Path: "logs/" + time.Now().Format("2006-01-02T15:04")}
	logReverse  *bool
	anyLogFound = false
)

type fileLog struct {
	Name string
	Path string
}

func splashScreen() {

	pterm.DefaultBigText.WithLetters(
		putils.LettersFromStringWithStyle("K", pterm.FgBlue.ToStyle()),
		putils.LettersFromStringWithStyle("Logs", pterm.FgWhite.ToStyle())).
		Render() // Render the big text to the terminal

	pterm.DefaultParagraph.Printfln("Version: %s", BuildVersion)
}

func configLogPath() {
	if *customLogPath != "" {
		fileLogs.Path = *customLogPath
	}
}

func configClient() {

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// create the client
	client, err = kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
}

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

	pterm.Info.Printfln("Using Namespace: %s", pterm.Green(*namespace))
}

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

func listPods(namespace string) {

	pods, err := client.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
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
		pterm.Error.Printfln("No pods found in namespace %s", namespace)
		return
	}

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

	if len(selectedPods) == 0 {
		pterm.Error.Printfln("No pods selected")
		return
	}

	for _, podName := range selectedPods {
		var podList v1.PodList
		podList.Items = append(podList.Items, podMap[podName])
		getPodLogs(namespace, podList)
	}
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

func getPodLogs(namespace string, pods v1.PodList) {

	for _, pod := range pods.Items {
		pterm.Success.Printfln("Found pod %s \n", pod.Name)
		podTree := pterm.TreeNode{Text: pod.Name}

		// print each container in the pod
		for _, container := range pod.Spec.Containers {

			// get logs for the container
			req := client.CoreV1().Pods(namespace).GetLogs(pod.Name, &v1.PodLogOptions{
				Container: container.Name,
			})

			// save logs to file
			logs, err := req.Stream(context.Background())
			if err != nil {
				pterm.Error.Printfln("Error getting logs for container %s\n%v", container.Name, err)
				break
				//panic(err.Error())
			}

			fileLogs.Name = fmt.Sprintf("%s-%s.log", pod.Name, container.Name)
			saveLog(logs)

			containerTree := []pterm.TreeNode{{Text: container.Name}}
			podTree.Children = append(podTree.Children, containerTree...)
		}
		pterm.DefaultTree.WithRoot(podTree).Render()
	}
}

func findPodByLabel(namespace string, label string) {

	pterm.Info.Printfln("Getting pods in namespace %s with label %s\n\n", pterm.Green(namespace), pterm.Green(label))
	spinner1, _ := pterm.DefaultSpinner.Start()

	pods, err := client.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: label,
	})
	if statusError, isStatus := err.(*errors.StatusError); isStatus {
		fmt.Printf("Error getting pods in namespace %s: %v\n",
			namespace, statusError.ErrStatus.Message)
	}
	if err != nil {
		panic(err.Error())
	}

	// if pods are not found print message
	if len(pods.Items) == 0 {
		pterm.Error.Printfln("No pods found in namespace %s with label %s\n", namespace, label)
		spinner1.Stop()
		return
	}
	getPodLogs(namespace, *pods)
	spinner1.Stop()
}

func saveLog(logs io.ReadCloser) {
	anyLogFound = true

	defer func(logs io.ReadCloser) {
		err := logs.Close()
		if err != nil {
			panic(err.Error())
		}
	}(logs)

	// Create the log file
	if err := os.MkdirAll(fileLogs.Path, 0755); err != nil {
		panic(err.Error())
	}
	logFilePath := filepath.Join(fileLogs.Path, fileLogs.Name)
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

	// Read and write logs in chunks
	buf := make([]byte, 4096) // 4KB chunks
	for {
		n, err := logs.Read(buf)
		if err != nil && err != io.EOF {
			panic(err.Error())
		}
		if n == 0 {
			break
		}
		if _, err := logFile.Write(buf[:n]); err != nil {
			panic(err.Error())
		}
	}

	// If logReverse is enabled, reverse the lines in the file
	if *logReverse {
		reverseLogFileInChunks(logFilePath)
	}
}

func reverseLogFileInChunks(filePath string) {
	// Open the file for reading
	file, err := os.Open(filePath)
	if err != nil {
		panic(err.Error())
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			panic(err.Error())
		}
	}(file)

	// Read the entire file into memory in chunks
	var content []byte
	buf := make([]byte, 4096) // 4KB chunks
	for {
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			panic(err.Error())
		}
		if n == 0 {
			break
		}
		content = append(content, buf[:n]...)
	}

	// Split the content into lines and reverse the order
	lines := bytes.Split(content, []byte("\n"))
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}

	// Join the reversed lines back into a single byte slice
	reversedContent := bytes.Join(lines, []byte("\n"))

	// Write the reversed content back to the file in chunks
	err = os.WriteFile(filePath, reversedContent, 0644)
	if err != nil {
		panic(err.Error())
	}
}

var rootCmd = &cobra.Command{
	Use:   "klogs",
	Short: "Get logs from pods with a specific label",
	Long: `Get logs from pods with a specific label in a namespace and save them to a file.
If no namespace is provided, the command will use the current context in the kubeconfig file.
If no label is provided, the command will list all pods in the namespace and prompt the user to select one. Collect all the logs even if the pod has multiple containers.
If no log path is provided, the logs will be saved in the "logs/datetime" directory in the current working directory.`,

	Run: func(cmd *cobra.Command, args []string) {

		splashScreen()
		configLogPath()
		configClient()
		configNamespace()

		if len(*labels) == 0 {
			listPods(*namespace)
		}

		for _, l := range *labels {
			findPodByLabel(*namespace, l)
		}

		if anyLogFound {
			pterm.Info.Printfln("Logs saved to %s", fileLogs.Path)
		}
	},
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
	customLogPath = rootCmd.Flags().StringP("logpath", "p", "", "Custom log path")
	logReverse = rootCmd.Flags().BoolP("reverse", "r", false, "Write logs in reverse order (date descending)")
	kubeconfig = rootCmd.Flags().String("kubeconfig", "", "(optional) Absolute path to the kubeconfig file")

	if home := homedir.HomeDir(); home != "" && *kubeconfig == "" {
		*kubeconfig = filepath.Join(home, ".kube", "config")
	} else {
		pterm.Fatal.Printfln("Kubeconfig not found, please provide a kubeconfig file with --kubeconfig")
	}
}
