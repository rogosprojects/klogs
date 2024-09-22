package cmd

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	v1 "k8s.io/api/core/v1"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

func startTviewApp() {
	app = tview.NewApplication()
	headerBox := tview.NewButton(fmt.Sprintf("Context: %s - Namespace: %s - Hit Enter or Esc to close", kubeCtx, *namespace)).SetBackgroundColorActivated(tcell.ColorDefault).SetSelectedFunc(func() {
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

func updateMonitoredPodBox(monitoredPodBox *tview.Flex) {
	startLoopIcon := []string{" .🚀", " ..🚀", " ...🚀"}
	i := 0

	tick := time.NewTicker(updateMonitoredPodBoxFreq)
	defer tick.Stop()

	for {

		app.QueueUpdateDraw(func() {
			monitoredPodBox.Clear()

			if len(monitoredPods) == 0 {
				monitoredPodBox.AddItem(tview.NewTextView().SetText("No pods being monitored").SetTextColor(tcell.ColorRed), 0, 1, false)
				return
			}
			updatedAt := fmt.Sprintf("[%s] Monitoring %s", time.Now().Format("15:04:05"), startLoopIcon[i])
			tree := tview.NewTreeView().SetRoot(tview.NewTreeNode(updatedAt).SetColor(tcell.ColorGreen))

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
		i++

		if i > 2 {
			i = 0
		}

		<-tick.C

	}
}

func updateSizeFileBox(logSizeBox *tview.Flex, logFiles *map[string]*os.File) {
	tick := time.NewTicker(updateSizeFileBoxFreq)
	defer tick.Stop()

	for {
		<-tick.C

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
	tick := time.NewTicker(checkNewPodsFreq)
	defer tick.Stop()

	for {
		<-tick.C
		addPodsToMonitor(getPodListByFlags(false))
	}
}

func addPodsToMonitor(podList v1.PodList) {

	for _, pod := range podList.Items {
		if pod.Status.Phase == v1.PodRunning {
			//check if pod is already being monitored
			if _, ok := (monitoredPods)[pod.Name]; !ok {

				go updateLiveBox(fmt.Sprintf("Found Pod: %s\n", pod.Name))
				(monitoredPods)[pod.Name] = tview.NewTreeView().SetRoot(tview.NewTreeNode(pod.Name).SetColor(tcell.ColorYellow))
				podsChannel <- pod
			}
		}
	}

}

func updateLiveBox(text string) {
	liveBox.SetText(text).SetTextAlign(tview.AlignCenter)
	time.Sleep(5 * time.Second)
	//clear liveBox if text is the same
	if liveBox.GetText(false) == text {
		liveBox.Clear()
	}
}
