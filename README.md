[![Go Report Card](https://goreportcard.com/badge/github.com/bitnami/gonit)](https://goreportcard.com/report/github.com/bitnami/gonit)
[![CI](https://github.com/bitnami/gonit/actions/workflows/main.yml/badge.svg)](https://github.com/bitnami/gonit/actions/workflows/main.yml)

# gonit

_gonit_ is an Apache 2.0 drop in replacement for [monit](https://mmonit.com/monit/).

Currently, it only supports a subset of its configuration settings and only process type checks.

It requires Go 1.8 (or newer) to build.

## Installation

```console
$ go get github.com/bitnami/gonit/...
```

## Building from source

```console
$ git clone https://github.com/bitnami/gonit.git
$ cd gonit
$ make build
+ build
*** Gonit binary created under ./dist/gonit/gonit ***
```

## Basic usage

You can check gonit's basic usage options by invoking its help menu:

```console
$ gonit -h
Usage:
  gonit [flags]
  gonit [command]

Available Commands:
  monitor     Monitor service
  quit        Terminate the execution of a running daemon
  reload      Reinitialize tool
  restart     Restart service
  start       Start service
  status      Print full status information for each service
  stop        Stop service
  summary     Print short status information for each service
  unmonitor   Unmonitor service

Flags:
  -c, --controlfile file        Use this control file (default "/etc/gonit/gonitrc")
  -d, --daemonize n             Run as a daemon once per n seconds
  -I, --foreground              Do not run in background (needed for run from init)
  -l, --logfile file            Print log information to this file. (default "/var/log/gonit.log")
  -p, --pidfile pidfile         Use this pidfile in daemon mode (default "/var/run/gonit.pid")
  -S, --socketfile socketfile   Use this socketfile to listen for requests in daemon mode (default "/var/run/gonit.sock")
  -s, --statefile file          Set the file gonit should write state information to (default "/var/lib/gonit/state")
  -v, --verbose                 Verbose mode, work noisy (diagnostic output)

Use "gonit [command] --help" for more information about a command.
```

## License

Copyright &copy; 2025 Broadcom. The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.

You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and limitations under the License.
