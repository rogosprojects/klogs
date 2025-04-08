/*
Package cmd is the entry point for the command line tool. It defines the root command and its flags.
*/
package cmd

import (
	"github.com/rivo/tview"
	"os"
	"sync"
	"time"

	"github.com/pterm/pterm"
	"github.com/pterm/pterm/putils"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
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
	kubeCtx                string
	footerText             []string
)

const (
	fileNameSeparator         = "__"
	checkNewPodsFreq          = 3 * time.Second
	updateMonitoredPodBoxFreq = 1 * time.Second
	updateSizeFileBoxFreq     = 3 * time.Second
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

		getClusterInfo(namespace, &kubeCtx)
		podList := getPodListByFlags(true)

		// start multithreaded goroutines

		// process pods
		wg.Add(1)
		go processPods(&wg)
		addPodsToMonitor(podList)

		if *follow {
			pterm.Info.Printfln("Streaming" + pterm.Green(" ON"))
			go checkNewPods(&wg)
			startTviewApp()
		} else {
			pterm.Info.Printfln("Streaming" + pterm.Green(" OFF"))
			close(podsChannel) // read only channel
			wg.Wait()
			//fmt.Printf("DONE. Closing channel\n")
		}

		footer()
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
	logPath = rootCmd.Flags().StringP("logpath", "p", defaultLogPath, "Custom log path")
	kubeconfig = rootCmd.Flags().String("kubeconfig", "", "(optional) Absolute path to the kubeconfig file")
	allPods = rootCmd.Flags().BoolP("all", "a", false, "Get logs for all pods in the namespace")
	since = rootCmd.Flags().StringP("since", "s", "", "Only return logs newer than a relative duration like 5s, 2m, or 3h. Defaults to all logs.")
	tail = rootCmd.Flags().Int64P("tail", "t", -1, "Lines of the most recent log to save")
	follow = rootCmd.Flags().BoolP("follow", "f", false, "Specify if the logs should be streamed")
	printVersion = rootCmd.Flags().BoolP("version", "v", false, "Print the version of the tool")
	initContainer = rootCmd.Flags().BoolP("init", "i", false, "Get logs for init containers")
}
