# Stopwatch
Stopwatch is a web app for measuring time. 
It provides a simple UI with start/stop button and a timeline.
HTTP API can be used from command line via curl.

## Features
Stopwatch allows to view total elapsed time for today and all the past days.
Every time record (an interval between start and stop) is stored separately and can be later viewed on a timeline.
The app is good for measuring the time you spend working and for understanding what time of day you are most productive at.

## Database setup
Stopwatch saves data to MySQL database. The following table must be created before running stopwatch:

```sql
CREATE TABLE `sessions` (
  `start` bigint(20) DEFAULT NULL,
  `end` bigint(20) DEFAULT NULL,
  KEY `start` (`start`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8
```

## Build
Stopwatch is written in Go. Go compiler is required to build the app. 
You also need to `go get` the following dependencies:

* github.com/go-sql-driver/mysql - for working with MySQL database
* github.com/BurntSushi/toml - for parsing configuration file
* github.com/gorilla/websocket - for live updates in web UI via websockets

After installing Go and dependencies cd to stopwatch directory and run `go build`.
A single executable file **stopwatch** will be created.

## Configuration
Stopwatch config is in TOML format. There are three sections of config:
1. stopwatch - the core configuration. 

    Here you set the hour that should be considered start of the day and a path to log file

2. http - HTTP server configuration

    Here you set HTTP port for the server, path to UI files and a prefix for all stopwatch links

3. db - MySQL configuration. Host, port, user, password and database.

Here is an example config:

```
[stopwatch]
day_start_hour = 8  # 8am will be considered start of the day, use 24-hour format here
log = "/var/log/stopwatch/stopwatch.log"

[http]
port = 8090
href_prefix = ""
static_dir = "/usr/local/stopwatch/ui"  # path to HTML templates, css and Javasript files for UI

[db]
host = "localhost"
user = "stopwatch_test"
password = ""
database = "stopwatch_test"
```

## Running
To use stopwatch execute the **stopwatch** binary and provide a path to config file using the -config flag:

    ./stopwatch -config=path/to/config.toml

stopwatch will run in foreground, so it's better to daemonize it using a tool like systemd, launchd etc.
