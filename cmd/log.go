package cmd

import (
	"bufio"
	"context"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/pterm/pterm"
	"io"
	v1 "k8s.io/api/core/v1"
	"os"
	"path/filepath"
	"time"
)

func createLogFile(podName string, containerName string) *os.File {
	logName := fmt.Sprintf("%s%s%s.log", podName, fileNameSeparator, containerName)

	// Create the log file
	if err := os.MkdirAll(*logPath, 0755); err != nil {
		panic(err.Error())
	}
	logFilePath := filepath.Join(*logPath, logName)

	if _, err := os.Stat(logFilePath); err == nil {
		// File exists
		footerText = append(footerText, pterm.Warning.Sprintf("File %s exists. Appending.", logFilePath))
	}

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

// streamLog streams logs for the container
func streamLog(pod v1.Pod, container v1.Container, logFile *os.File, logOpts v1.PodLogOptions) {
	defer wg.Done()
	if *follow {
		defer func() {
			if _, ok := monitoredPods[pod.Name]; !ok {
				return
			}
			monitoredPods[pod.Name].GetRoot().SetColor(tcell.ColorRed).SetChildren(nil)
			footerText = append(footerText, pterm.Warning.Sprintf("[%s] Streaming logs ended prematurely for\n\tPod: %s\n\tContainer: %s", time.Now().Format("15-04-01"), pod.Name, container.Name))
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
