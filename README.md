[![Go Report Card](https://goreportcard.com/badge/github.com/bitnami/gonit)](https://goreportcard.com/report/github.com/bitnami/gonit)
[![Build Status](https://api.travis-ci.org/bitnami/gonit.svg?branch=master)](http://travis-ci.org/bitnami/gonit)

# gonit

_gonit_ is a GPL drop in replacement form [monit](https://mmonit.com/monit/).

Currently, it only supports a subset of its configuration settings and only process type checks.

## Installation

```
$> go get github.com/bitnami/gonit/...
```

## Building from source

```
$> git clone https://github.com/bitnami/gonit.git
$> cd gonit
$> make build
+ build
*** Gonit binary created under ./dist/gonit/gonit ***
```

## Basic usage

You can check gonit's basic usage options by invoking its help menu:

```
$> gonit -h
Usage:
  gonit [flags]
  gonit [command]

Available Commands:
  monitor     Monitor service
  quit        Terminate the executing of a running daemin
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

