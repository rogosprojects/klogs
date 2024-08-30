![Project Logo](/assets/logo-extended.jpeg)

# klogs
[![Go Report Card](https://goreportcard.com/badge/github.com/derailed/k9s?)](https://goreportcard.com/report/github.com/derailed/k9s)
[![GitHub release](https://img.shields.io/github/release/rogosprojects/klogs.svg)]()

Kubernetes batch log extractor written in GO. *Blazing fast. 🔥*

## Overview

`klogs` is fast Kubernetes log extractor written in Go. It simplifies the process of extracting logs from Kubernetes clusters, making it easier for developers and operations teams to monitor and debug applications running in Kubernetes environments.

## Features

- **Efficient Log Extraction**: Quickly collect logs from Kubernetes pods even if the pod has multiple containers.
- **Namespace Support**: Allows targeting logs within specific namespaces.
- **Label Filtering**: Extract logs from pods matching specific labels.
- **Multiple-Pods Log Download**: Supports downloading logs from multiple pods simultaneously, enhancing efficiency when dealing with large-scale deployments.
- **Output Flexibility**: Saves logs to a specified directory or outputs to date-based folder.
- **Follow Logs**: Stream logs in real-time for debugging and monitoring.

## Installation

### From binaries

Simply download [latest binaries](https://github.com/rogosprojects/klogs/releases/latest).

### From sources

To compile and install _klogs_ from sources, ensure you have Go installed on your system.
Then, clone the repository and build the binary:

```
go install github.com/rogosprojects/klogs@latest
```

## Usage
![Select Pods](/assets/klogs-select-pods.png)

```
Usage:
  klogs [flags]
```

| Flag            | Type        | Description                                                                         |
|-----------------|-------------|-------------------------------------------------------------------------------------|
| --kubeconfig    | string      | [optional] absolute path to the kubeconfig file                                     |
| -p, --logpath   | string      | [default:logs] Custom log path                                                      |
| -n, --namespace | string      | [default:current] Select namespace                                                  |
| -l, --label     | stringArray | [optional] Select label (or labels with multiple -l flags)                          |
| -a, --all       | boolean     | [default:false] Select all pods in namespace                                        |
| -s, --since     | string      | [optional] Only return logs newer than a relative duration. Examples: 1m, 2h, 2h45m |
| -t, --tail      | int         | [optional] Number of lines to show from the end of the logs                         |
| -f, --follow    | boolean     | [default:false] Stream logs in real-time                                            |
| -v, --version   |             | Print version information and exit                                                  |

## Features

* **Namespace Context**: If no namespace is provided, the command will use the current context in the kubeconfig file.
* **Pod Selection**: If no label is provided, the command will list all pods in the namespace and prompt the user to select one. It collects all the logs even if the pod has multiple containers.

***Example:***
  `klogs -n my-namespace -l app=my-app -l tier=backend`


* **Custom Log Path**: If no log path is provided, the logs will be saved in the "logs" directory in the current working directory.
* **All Pods Logging**: If the "all" flag is set, the logs will be saved for all pods in the namespace.


***Example:***
  `klogs -n my-namespace -a -p /path/to/logs`

* **Time-based Log Filtering**: If the "since" flag is set, only logs newer than the specified duration will be saved.

***Example:***
  `klogs -n my-namespace -l app=my-app -p /path/to/logs -s 5m`

* **Tail Log Lines**: If the "tail" flag is set, only the specified number of lines will be saved.

***Example:***
  `klogs -n my-namespace -l app=my-app -p /path/to/logs -s 5m -t 100`

* **Follow Logs**: If the "follow" flag is set, the logs will be streamed in real-time.

***Example:***
  `klogs -n my-namespace -l app=my-app -f`

## Contributing
Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.