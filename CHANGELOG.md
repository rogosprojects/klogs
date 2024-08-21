# Changelog

## [v1.1.18] - 2024-08-20
### :bug: Bug Fixes
- [`c9bc851`](https://github.com/rogosprojects/klogs/commit/c9bc851b99bd5442e2116c6bd0071b62522ee931) - handle incorrect kubeconfig file with Fatal msg *(commit by [@rogosprojects](https://github.com/rogosprojects))*
- [`21edee9`](https://github.com/rogosprojects/klogs/commit/21edee9a4fb92ca0056d13aee011c548797dd8bd) - avoid race conditions! *(commit by [@rogosprojects](https://github.com/rogosprojects))*
- [`38fa5b3`](https://github.com/rogosprojects/klogs/commit/38fa5b3c0ac060c37d3628bf6477d27e881d9347) - avoid invalid chars in folder name *(commit by [@rogosprojects](https://github.com/rogosprojects))*


## [v1.1.17] - 2024-08-14
### :wrench: Chores
- [`63f50cb`](https://github.com/rogosprojects/klogs/commit/63f50cb7f3aeb4e68b1b107f23844a3735d4d3b9) - print log saving path with --follow too *(commit by [@rogosprojects](https://github.com/rogosprojects))*


## [v1.1.16] - 2024-08-12
### :sparkles: New Features
- [`9371c86`](https://github.com/rogosprojects/klogs/commit/9371c862aad0d7ce3407f35e961af2f899de2afa) - add print version flag *(commit by [@rogosprojects](https://github.com/rogosprojects))*

### :bug: Bug Fixes
- [`13caa51`](https://github.com/rogosprojects/klogs/commit/13caa519e3305dac37608d85e51f25395b78dba6) - lower go minimum version required *(commit by [@rogosprojects](https://github.com/rogosprojects))*


## [v1.1.15] - 2024-08-12
### :sparkles: New Features
- [`de82e75`](https://github.com/rogosprojects/klogs/commit/de82e75baf64d603db5ada51eec5a846e13e1fdf) - press "q" button to exit follow mode *(commit by [@rogosprojects](https://github.com/rogosprojects))*

### :zap: Performance Improvements
- [`165e5f3`](https://github.com/rogosprojects/klogs/commit/165e5f3d0338ed9b7fc1f98c45a62594dbfbaf74) - improve spinner when updating *(commit by [@rogosprojects](https://github.com/rogosprojects))*

### :wrench: Chores
- [`ecdd8fe`](https://github.com/rogosprojects/klogs/commit/ecdd8fe3c128652e86e646043aa3c7a382c5585a) - differentiate msg on follow *(commit by [@davidecavestro](https://github.com/davidecavestro))*
- [`1bdcf0d`](https://github.com/rogosprojects/klogs/commit/1bdcf0d0313cb91a97ddc541ec8b0df285de3a2f) - generate release notes and update changelog *(commit by [@davidecavestro](https://github.com/davidecavestro))*
- [`0bd1656`](https://github.com/rogosprojects/klogs/commit/0bd1656979ee1fd83eb168ab9b10b2efd0e90f92) - add support for testing on push *(commit by [@davidecavestro](https://github.com/davidecavestro))*
- [`a4878ef`](https://github.com/rogosprojects/klogs/commit/a4878ef3ab2b1202750471b2c50701d40c642ea9) - removed unused dep *(commit by [@rogosprojects](https://github.com/rogosprojects))*
- [`dbee160`](https://github.com/rogosprojects/klogs/commit/dbee160545701bba0241983950f7f6260feb9988) - starting tty.Open only in case of follow. *(commit by [@rogosprojects](https://github.com/rogosprojects))*


## v1.1.14
New feature: `--follow` option to stream logs in real-time. This is a game-changer for debugging and monitoring. Improved goroutines to fetch logs from multiple pods/containers simultaneously. Faster. 🔥
## v1.1.13

Support for parallel log fetching. Fetch logs from multiple pods/containers simultaneously. This enhances efficiency when dealing with large-scale deployments. Show log size.

## v1.1.12

Faster. 🔥 Use "bufio" to read logs in chunks instead of line by line. This is especially useful when reading large logs. Removed the "--reverse" option as it is just overhead now.

## v1.1.11

Added "--since" option to fetch logs newer than a relative duration. Added "--tail" option to get only the specified number of lines from the end of the logs.

## v1.1.10

Added "--all" option to get all logs in current namespace.

## v1.1.9

Added "--reverse" option to write logs in reverse order (date descending)

## v1.1.6

pass BuildVersion from the build system

## v1.1.1

Initial public release
[v1.1.15]: https://github.com/rogosprojects/klogs/compare/v1.1.14...v1.1.15
[v1.1.16]: https://github.com/rogosprojects/klogs/compare/v1.1.15...v1.1.16
[v1.1.17]: https://github.com/rogosprojects/klogs/compare/v1.1.16...v1.1.17
[v1.1.18]: https://github.com/rogosprojects/klogs/compare/v1.1.17...v1.1.18
