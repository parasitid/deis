package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/deis/deis/deisctl/backend/fleet"
	"github.com/deis/deis/deisctl/client"
	"github.com/deis/deis/deisctl/utils"

	docopt "github.com/docopt/docopt-go"
)

const (
	// Version of deisctl client
	Version string = "1.0.1"
)

// main exits with the return value of Command(os.Args[1:]), deferring all logic to
// a func we can test.
func main() {
	os.Exit(Command(nil))
}

// Command executes the given deisctl command line.
func Command(argv []string) int {
	deisctlMotd := utils.DeisIfy("Deis Control Utility")
	usage := deisctlMotd + `
Usage: deisctl <command> [<args>...] [options]

Commands, use "deisctl help <command>" to learn more:
  install           install components, or the entire platform
  uninstall         uninstall components
  list              list installed components
  start             start compnents
  stop              stop components
  restart           stop, then start components
  scale             grow or shrink the number of routers
  journal           print the log output of a component
  config            set platform or component values
  refresh-units     refresh unit files from GitHub
  help              show the help screen for a command

Options:
  -h --help                   show this help screen
  --endpoint=<url>            etcd endpoint for fleet [default: http://127.0.0.1:4001]
  --etcd-cafile=<path>        etcd CA file authentication [default: ]
  --etcd-certfile=<path>      etcd cert file authentication [default: ]
  --etcd-key-prefix=<path>    keyspace for fleet data in etcd [default: /_coreos.com/fleet/]
  --etcd-keyfile=<path>       etcd key file authentication [default: ]
  --known-hosts-file=<path>   where to store remote fingerprints [default: ~/.ssh/known_hosts]
  --request-timeout=<secs>    seconds before a request is considered failed [default: 10.0]
  --strict-host-key-checking  verify SSH host keys [default: true]
  --tunnel=<host>             SSH tunnel for communication with fleet and etcd [default: ]
  --version                   print the version of deisctl
`
	// pre-parse command-line arguments
	argv, helpFlag := parseArgs(argv)
	// give docopt an optional final false arg so it doesn't call os.Exit()
	args, err := docopt.Parse(usage, argv, false, Version, false, false)
	if err != nil || len(args) == 0 {
		if helpFlag {
			fmt.Print(usage)
			return 0
		} else {
			return 1
		}
	}
	command := args["<command>"]
	setTunnel := true
	// "--help" and "refresh-units" doesn't need SSH tunneling
	if helpFlag || command == "refresh-units" {
		setTunnel = false
	}
	setGlobalFlags(args, setTunnel)
	// clean up the args so subcommands don't need to reparse them
	argv = removeGlobalArgs(argv)
	// construct a client
	c, err := client.NewClient("fleet")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return 1
	}
	// Dispatch the command, passing the argv through so subcommands can
	// re-parse it according to their usage strings.
	switch command {
	case "list":
		err = c.List(argv)
	case "scale":
		err = c.Scale(argv)
	case "start":
		err = c.Start(argv)
	case "restart":
		err = c.Restart(argv)
	case "stop":
		err = c.Stop(argv)
	case "status":
		err = c.Status(argv)
	case "journal":
		err = c.Journal(argv)
	case "install":
		err = c.Install(argv)
	case "uninstall":
		err = c.Uninstall(argv)
	case "config":
		err = c.Config(argv)
	case "refresh-units":
		err = c.RefreshUnits(argv)
	case "help":
		fmt.Print(usage)
		return 0
	default:
		fmt.Println(`Found no matching command, try "deisctl help"
Usage: deisctl <command> [<args>...] [options]`)
		return 1
	}
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return 1
	}
	return 0
}

// isGlobalArg returns true if a string looks like it is a global deisctl option flag,
// such as "--tunnel".
func isGlobalArg(arg string) bool {
	prefixes := []string{
		"--endpoint=",
		"--etcd-key-prefix=",
		"--etcd-keyfile=",
		"--etcd-certfile=",
		"--etcd-cafile=",
		// "--experimental-api=",
		"--known-hosts-file=",
		"--strict-host-key-checking=",
		"--request-timeout=",
		"--tunnel",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(arg, p) {
			return true
		}
	}
	return false
}

// parseArgs returns the provided args with "--help" as the last arg if need be,
// and a boolean to indicate whether help was requested.
func parseArgs(argv []string) ([]string, bool) {
	if argv == nil {
		argv = os.Args[1:]
	}

	if len(argv) == 1 {
		// rearrange "deisctl --help" as "deisctl help"
		if argv[0] == "--help" || argv[0] == "-h" {
			argv[0] = "help"
		}
	}

	if len(argv) >= 2 {
		// rearrange "deisctl help <command>" as "deisctl <command> --help"
		if argv[0] == "help" || argv[0] == "--help" || argv[0] == "-h" {
			argv = append(argv[1:], "--help")
		}
	}

	helpFlag := false
	for _, a := range argv {
		if a == "help" || a == "--help" || a == "-h" {
			helpFlag = true
			break
		}
	}

	return argv, helpFlag
}

// removeGlobalArgs returns the given args without any global option flags, to make
// re-parsing by subcommands easier.
func removeGlobalArgs(argv []string) []string {
	v := make([]string, 0)
	for _, a := range argv {
		if !isGlobalArg(a) {
			v = append(v, a)
		}
	}
	return v
}

// setGlobalFlags sets fleet provider options based on deisctl global flags.
func setGlobalFlags(args map[string]interface{}, setTunnel bool) {
	fleet.Flags.Endpoint = args["--endpoint"].(string)
	fleet.Flags.EtcdKeyPrefix = args["--etcd-key-prefix"].(string)
	fleet.Flags.EtcdKeyFile = args["--etcd-keyfile"].(string)
	fleet.Flags.EtcdCertFile = args["--etcd-certfile"].(string)
	fleet.Flags.EtcdCAFile = args["--etcd-cafile"].(string)
	//fleet.Flags.UseAPI = args["--experimental-api"].(bool)
	fleet.Flags.KnownHostsFile = args["--known-hosts-file"].(string)
	fleet.Flags.StrictHostKeyChecking = args["--strict-host-key-checking"].(bool)
	timeout, _ := strconv.ParseFloat(args["--request-timeout"].(string), 64)
	fleet.Flags.RequestTimeout = timeout
	if setTunnel == true {
		tunnel := args["--tunnel"].(string)
		if tunnel != "" {
			fleet.Flags.Tunnel = tunnel
		} else {
			fleet.Flags.Tunnel = os.Getenv("DEISCTL_TUNNEL")
		}
	}
}
