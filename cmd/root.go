/*
Package cmd is the entry point for the command line tool. It defines the root command and its flags.
*/
package cmd

import (
	"bufio"
	"context"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"io"
	"k8s.io/client-go/util/homedir"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
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
	allPods, initContainer *bool
	defaultLogPath         = "logs/" + time.Now().Format("2006-01-02T15-04")
	wg                     sync.WaitGroup
	logFiles               = make(map[string]*os.File)
	mu                     sync.Mutex
	app                    *tview.Application
	monitoredPods          = make(map[string]*tview.TreeView)
	liveBox                = tview.NewTextView()
	podsChannel            = make(chan v1.Pod, 50)
)

const (
	fileNameSeparator = "__"
)

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
	config.Burst = 100

	// create the client
	client, err = kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
}

// getClusterInfo configures the namespace to use
func getClusterInfo(kubeconfig string) (ns string, ctx string) {

	config, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	if *namespace == "" {
		*namespace = config.Contexts[config.CurrentContext].Namespace

		if len(ns) == 0 {
			ns = "default"
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

	return *namespace, config.CurrentContext
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

// listAllPods lists all the ready pods in the namespace
func listAllPods() v1.PodList {
	_podList := v1.PodList{}

	var podMap = make(map[string]v1.Pod)
	var podNames []string
	for _, pod := range kGetAllPods(*namespace).Items {
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

func printLogSize() {
	if (len(logFiles)) == 0 {
		pterm.Error.Printfln("No logs saved")
		return
	}
	pterm.Info.Printfln("Logs saved to " + pterm.Green(*logPath))

	tableData := pterm.TableData{{"Pod", "Container", "Size"}}

	var previousPod string

	// sort output
	logNames := make([]string, 0, len(logFiles))
	for k := range logFiles {
		logNames = append(logNames, k)
	}
	sort.Strings(logNames)

	for k := range logNames {
		fileName := logNames[k]
		fileInfo, err := logFiles[fileName].Stat()
		if err != nil {
			continue
		}
		podName, containerName := strings.Split(fileName, fileNameSeparator)[0], strings.Split(fileName, fileNameSeparator)[1]
		containerName = strings.TrimSuffix(containerName, ".log")

		podNameLabelColor := podName
		if podName == previousPod {
			podNameLabelColor = pterm.Gray(podName)
		}
		tableData = append(tableData, []string{podNameLabelColor, containerName, convertBytes(fileInfo.Size())})
		previousPod = podName
	}
	err := pterm.DefaultTable.WithHasHeader().WithBoxed().WithData(tableData).Render()
	if err != nil {
		pterm.Error.Printfln("Error rendering table")
	}
}

// streamLog streams logs for the container
func streamLog(pod v1.Pod, container v1.Container, logFile *os.File, logOpts v1.PodLogOptions) {
	defer wg.Done()
	if *follow {
		defer func() {
			if _, ok := monitoredPods[pod.Name]; !ok {
				return
			}
			monitoredPods[pod.Name].GetRoot().SetColor(tcell.ColorRed).SetChildren(nil)
			//pterm.Warning.Printfln("Streaming logs ended prematurely for Pod: %s, Container: %s", pod.Name, container.Name)
		}()
	}

	logOpts.Container = container.Name
	// get logs for the container
	req := client.CoreV1().Pods(*namespace).GetLogs(pod.Name, &logOpts)

	// get logs
	logs, err := req.Stream(context.Background())
	if err != nil {
		//pterm.Error.Printfln("Error getting logs for container %s\n%v", container.Name, err)
		return
	}
	defer func(logs io.ReadCloser) {
		err := logs.Close()
		if err != nil {
			panic(err.Error())
		}
	}(logs)

	writeLogToDisk(logs, logFile)
}

func createLogFile(podName string, containerName string) *os.File {
	logName := fmt.Sprintf("%s%s%s.log", podName, fileNameSeparator, containerName)

	// Create the log file
	if err := os.MkdirAll(*logPath, 0755); err != nil {
		panic(err.Error())
	}
	logFilePath := filepath.Join(*logPath, logName)

	//if _, err := os.Stat(logFilePath); err == nil {
	//	// File exists
	//	fmt.Printf("File %s exists.\n Appending.", logFilePath)
	//}
	// If the file doesn't exist, create it, or append to the file
	logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err.Error())
	}

	logFiles[logName] = logFile

	return logFile
}

// writeLogToDisk writes logs to disk
func writeLogToDisk(logs io.ReadCloser, logFile *os.File) {
	reader := bufio.NewReader(logs)

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
func updateMonitoredPodBox(monitoredPodBox *tview.Flex) {
	startLoopIcon := []string{" 🚀", " 🚀🚀", " 🚀🚀🚀"}
	i := 0
	for {
		app.QueueUpdateDraw(func() {
			monitoredPodBox.Clear()

			if len(monitoredPods) == 0 {
				monitoredPodBox.AddItem(tview.NewTextView().SetText("No pods being monitored").SetTextColor(tcell.ColorRed), 0, 1, false)
				return
			}
			updatedAt := time.Now().Format("15:04:05")
			tree := tview.NewTreeView().SetRoot(tview.NewTreeNode(updatedAt + startLoopIcon[i]).SetColor(tcell.ColorGreen))

			// sort output
			podNames := make([]string, 0, len(monitoredPods))
			for k := range monitoredPods {
				podNames = append(podNames, k)
			}
			sort.Strings(podNames)

			for _, k := range podNames {
				if (monitoredPods)[k] == nil {
					continue
				}
				tree.GetRoot().AddChild((monitoredPods)[k].GetRoot())
				//monitoredPodBox.AddItem((*monitoredPods)[k], 3, 1, false)
			}
			for podName, tree := range monitoredPods {
				if tree.GetRoot().GetColor() == tcell.ColorRed {
					delete(monitoredPods, podName)
				} else if tree.GetRoot().GetColor() == tcell.ColorYellow {
					tree.GetRoot().SetColor(tcell.ColorGreen)
				} else if tree.GetRoot().GetColor() == tcell.ColorGreen {
					tree.GetRoot().SetColor(tcell.ColorBlue)
				}
			}
			monitoredPodBox.AddItem(tree, 0, 1, false)
		})
		time.Sleep(1 * time.Second)
		i++

		if i > 2 {
			i = 0
		}
	}
}

func updateSizeFileBox(logSizeBox *tview.Flex, logFiles *map[string]*os.File) {
	for {
		app.QueueUpdateDraw(func() {
			logSizeBox.Clear()

			if len(*logFiles) == 0 {
				logSizeBox.AddItem(tview.NewTextView().SetText("No logs saved").SetTextColor(tcell.ColorRed), 0, 1, false)
				return
			}

			table := tview.NewTable().
				SetBorders(false)
			table.SetCell(0, 0, tview.NewTableCell("Pod").SetSelectable(false).SetTextColor(tcell.ColorBlue))
			table.SetCell(0, 1, tview.NewTableCell("Container").SetSelectable(false).SetTextColor(tcell.ColorBlue))
			table.SetCell(0, 2, tview.NewTableCell("Size").SetSelectable(false).SetTextColor(tcell.ColorBlue))

			var previousPod string
			var cellStart int

			// sort output
			logNames := make([]string, 0, len(*logFiles))
			for k := range *logFiles {
				logNames = append(logNames, k)
			}
			sort.Strings(logNames)

			for k := range logNames {
				fileName := logNames[k]
				log := (*logFiles)[fileName]
				fileInfo, err := log.Stat()
				if err != nil {
					continue
				}
				podName, containerName := strings.Split(fileName, fileNameSeparator)[0], strings.Split(fileName, fileNameSeparator)[1]
				containerName = strings.TrimSuffix(containerName, ".log")

				color := tcell.ColorWhite
				if podName == previousPod {
					color = tcell.ColorGray
				}
				table.SetCell(cellStart+1, 0, tview.NewTableCell(podName).SetSelectable(false).SetTextColor(color))
				table.SetCellSimple(cellStart+1, 1, containerName)
				table.SetCellSimple(cellStart+1, 2, convertBytes(fileInfo.Size()))
				cellStart++
				previousPod = podName
			}

			logSizeBox.AddItem(table, 0, 1, false)

		})
		time.Sleep(2 * time.Second)
	}
}
func convertBytes(bytes int64) string {
	if bytes == 0 {
		return fmt.Sprintf("0 B")
	}
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%d KB", bytes/1024)
	}
	return fmt.Sprintf("%d MB", bytes/1024/1024)
}

var rootCmd = &cobra.Command{
	Use:   "klogs",
	Short: "Get logs from Pods, super fast! 🚀",
	Long: `klogs is a CLI tool to get logs from Kubernetes Pods.
It is designed to be fast and efficient, and can get logs from multiple Pods/Containers at once. Blazing fast. 🔥`,

	Run: func(cmd *cobra.Command, args []string) {

		if *printVersion {
			pterm.Info.Printfln("Version: %s", BuildVersion)
			os.Exit(0)
		}

		splashScreen()
		configClient()

		ns, ctx := getClusterInfo(*kubeconfig)

		var podList v1.PodList
		//var filesChannel = make(chan os.File, 50)

		if len(*labels) == 0 {
			pterm.Info.Println("Getting all Pods")
			podList = listAllPods()
		} else {
			for _, l := range *labels {
				pterm.Info.Printf("Getting Pods with label %s\n\n", pterm.Green(l))
				podList.Items = append(podList.Items, findPodByLabel(l).Items...)
			}
		}

		// start multithreaded goroutines

		// process pods
		wg.Add(1)
		go processPods(&wg)
		addPodsToMonitor(podList)

		if *follow {
			pterm.Info.Printfln("Streaming ON")
			go checkNewPods(&wg)
			startTviewApp(ns, ctx)
		} else {
			pterm.Info.Printfln("Streaming OFF")
			close(podsChannel) // read only channel
			wg.Wait()
			//fmt.Printf("DONE. Closing channel\n")
		}

		printLogSize()
	},
}

func startTviewApp(ns string, ctx string) {
	app = tview.NewApplication()
	headerBox := tview.NewButton(fmt.Sprintf("[CONTEXT: %s] [NAMESPACE: %s] Hit Enter or Esc to close", ctx, ns)).SetBackgroundColorActivated(tcell.ColorDefault).SetSelectedFunc(func() {
		app.Stop()
	}).SetExitFunc(func(key tcell.Key) {
		app.Stop()
	})
	monitoredPodBox := tview.NewFlex()

	monitoredPodBox.SetDirection(tview.FlexRow).
		SetTitle(" Monitored Pods ").
		SetBorder(true).
		SetTitleAlign(tview.AlignCenter).
		SetTitleColor(tcell.ColorGreen)

	logSizeBox := tview.NewFlex()
	logSizeBox.SetDirection(tview.FlexRow).
		SetTitle(" Logs ").
		SetBorder(true).
		SetTitleAlign(tview.AlignCenter).
		SetTitleColor(tcell.ColorGreen)

	go updateMonitoredPodBox(monitoredPodBox)
	go updateSizeFileBox(logSizeBox, &logFiles)

	grid := tview.NewGrid().
		SetBorders(false).
		SetRows(1, 0, 1).
		SetColumns(60, 0).
		AddItem(headerBox, 0, 0, 1, 2, 0, 0, true).
		AddItem(monitoredPodBox, 1, 0, 1, 1, 0, 0, false).
		AddItem(logSizeBox, 1, 1, 1, 1, 0, 0, false).
		AddItem(liveBox, 2, 0, 1, 2, 0, 0, false)

	if err := app.SetRoot(grid, true).EnableMouse(false).Run(); err != nil {
		panic(err)
	}
}

func processPods(wg *sync.WaitGroup) {
	defer wg.Done()
	for p := range podsChannel {
		//fmt.Printf("Processing Pod: %s\n Remaining: %d", p.Name, len(podsChannel))
		getPodLogsV2(p, getLopOpts())
	}
}

func checkNewPods(wg *sync.WaitGroup) {
	defer wg.Done()
	tick := time.NewTicker(1 * time.Second)
	defer tick.Stop()

	for {
		<-tick.C

		if *allPods {
			addPodsToMonitor(kGetAllPods(*namespace))
		} else if len(*labels) > 0 {
			for _, l := range *labels {
				addPodsToMonitor(findPodByLabel(l))
			}
		} else {
			return
		}

	}
}

func addPodsToMonitor(podList v1.PodList) {

	for _, pod := range podList.Items {
		if pod.Status.Phase == v1.PodRunning {
			//check if pod is already being monitored
			if _, ok := (monitoredPods)[pod.Name]; !ok {
				liveBox.Clear()
				liveBox.SetText(fmt.Sprintf("Found Pod: %s\n", pod.Name))
				//fmt.Printf("Found Pod: %s\n", pod.Name)
				(monitoredPods)[pod.Name] = tview.NewTreeView().SetRoot(tview.NewTreeNode(pod.Name).SetColor(tcell.ColorYellow))
				podsChannel <- pod
			}
		}
	}

	//fmt.Printf("Pod %s already being monitored\n", pod.Name)
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
	initContainer = rootCmd.Flags().BoolP("init", "i", false, "Get logs for init containers")

}
