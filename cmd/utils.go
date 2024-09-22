package cmd

import (
	"atomicgo.dev/keyboard/keys"
	"fmt"
	"github.com/pterm/pterm"
	v1 "k8s.io/api/core/v1"
	"sort"
	"strings"
)

func getPodListByFlags(verbose bool) v1.PodList {
	podList := v1.PodList{}
	if *allPods {
		if verbose {
			pterm.Info.Println("Getting all Pods")
		}
		podList, _ = listAllPods()
	} else if len(*labels) > 0 {
		for _, l := range *labels {
			if verbose {
				pterm.Info.Printf("Getting Pods with label %s\n\n", pterm.Green(l))
			}
			podList.Items = append(podList.Items, findPodByLabel(l).Items...)
		}
	} else {
		podList = interactivePodSelect()
	}
	return podList
}

// listAllPods lists all the ready pods in the namespace
func listAllPods() (podList v1.PodList, podMap map[string]v1.Pod) {

	_podList := v1.PodList{}
	_podMap := make(map[string]v1.Pod)

	for _, pod := range kGetAllPods(*namespace).Items {
		// is the pod ready?
		for _, condition := range pod.Status.Conditions {
			if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
				_podMap[pod.Name] = pod
				_podList.Items = append(_podList.Items, pod)
				break
			}
		}
	}

	if len(_podList.Items) == 0 {
		pterm.Error.Printfln("No pods found in namespace %s", *namespace)
		return _podList, _podMap
	}

	return _podList, _podMap
}

// interactivePodSelect shows an interactive multiselect printer with the pod names
func interactivePodSelect() v1.PodList {
	_, _podMap := listAllPods()

	// get all _podMap keys
	_podNames := make([]string, 0, len(_podMap))
	for k := range _podMap {
		_podNames = append(_podNames, k)
	}

	// Create a new interactive multiselect printer with the options
	// Disable the filter and set the keys for confirming and selecting options
	printer := pterm.DefaultInteractiveMultiselect.
		WithOptions(_podNames).
		WithFilter(false).
		WithKeyConfirm(keys.Enter).
		WithKeySelect(keys.Space).
		WithMaxHeight(15).
		WithDefaultText("Select Pods to get logs")

	// Show the interactive multiselect and get the selected options
	selectedPods, _ := printer.Show()

	_podList := v1.PodList{}
	if len(selectedPods) == 0 {
		pterm.Error.Printfln("No pods selected")
		return _podList
	}

	// collect info only for the selected pods
	for _, podName := range selectedPods {
		_podList.Items = append(_podList.Items, _podMap[podName])
	}
	return _podList
}

func footer() {
	if (len(logFiles)) == 0 {
		pterm.Error.Printfln("No logs saved")
		return
	}
	pterm.Info.Printfln("Logs saved to " + pterm.Green(*logPath))

	tableData := pterm.TableData{{"Pod", "Container", "Size"}}

	// sort output
	logNames := make([]string, 0, len(logFiles))
	for k := range logFiles {
		logNames = append(logNames, k)
	}
	sort.Strings(logNames)

	var previousPod string
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

	if len(footerText) > 0 {
		fmt.Println("Please note:")
		for _, line := range footerText {
			fmt.Println(line)
		}
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
