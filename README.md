# klogs

![logo](./path/to/logo.png) <!-- Replace with the actual logo path -->

## Overview

**klogs** is a blazing-fast Kubernetes log extractor written in Go. It simplifies the process of extracting logs from Kubernetes clusters, making it easier for developers and operations teams to monitor and debug applications running in Kubernetes environments.

## Features

- **Efficient Log Extraction**: Quickly extracts logs from specified Kubernetes pods.
- **Namespace Support**: Allows targeting logs within specific namespaces.
- **Label Filtering**: Extract logs from pods matching specific labels.
- **Output Flexibility**: Saves logs to a specified directory or outputs to date-based folder.

## Installation

To install klogs, ensure you have Go installed on your system. Then, clone the repository and build the binary:

```
go install https://github.com/rogosprojects/klogs@latest
```

## Usage

`Usage: klogs -n <namespace> -l <label>`

* If no namespace is provided, the command will use the current context in the kubeconfig file.
* If no label is provided, the command will list all pods in the namespace and prompt the user to select one.
* If logpath is provided, the logs will be saved to that path instead of the default logs/ directory.
