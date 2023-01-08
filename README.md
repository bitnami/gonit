[![Go Report Card](https://goreportcard.com/badge/github.com/bitnami/gonit)](https://goreportcard.com/report/github.com/bitnami/gonit)
[![CI](https://github.com/bitnami/gonit/actions/workflows/main.yml/badge.svg)](https://github.com/bitnami/gonit/actions/workflows/main.yml)

# gonit

_gonit_ is a GPLv2 drop in replacement for [monit](https://mmonit.com/monit/).

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

Copyright &copy; 2023 Bitnami

This program is free software; you can redistribute it and/or modify it under the terms of the GNU General Public License as published by the Free Software Foundation; either version 2 of the License, or any later version.

This program is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU General Public License for more details.
