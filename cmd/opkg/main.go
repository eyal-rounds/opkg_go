package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/oe-mirrors/opkg_go/internal/pkgmgr"
)

var (
	version   = "dev"
	buildTime = ""
)

func main() {
	var conf string
	flag.StringVar(&conf, "conf", defaultConfig(), "Path to opkg.conf")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [options] <command> [args]\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "Commands:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  update                Refresh package indexes\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  list                  List available packages\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  list-installed        List installed packages\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  info <pkg>            Show package metadata\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  install <pkg>         Download a package into the cache\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  version               Print version information\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	ctx := context.Background()

	switch args[0] {
	case "version", "--version", "-V":
		printVersion()
		return
	case "list-installed":
		manager, err := pkgmgr.New(conf)
		if err != nil {
			fatal(err)
		}
		for _, line := range manager.List(true) {
			fmt.Println(line)
		}
	case "update":
		manager, err := pkgmgr.New(conf)
		if err != nil {
			fatal(err)
		}
		if err := manager.Update(ctx); err != nil {
			fatal(err)
		}
		fmt.Println("Indexes updated.")
	case "list":
		manager, err := pkgmgr.New(conf)
		if err != nil {
			fatal(err)
		}
		if err := manager.Update(ctx); err != nil {
			fatal(err)
		}
		for _, line := range manager.List(false) {
			fmt.Println(line)
		}
	case "info":
		if len(args) < 2 {
			fatal(fmt.Errorf("info command expects a package name"))
		}
		manager, err := pkgmgr.New(conf)
		if err != nil {
			fatal(err)
		}
		if err := manager.Update(ctx); err != nil {
			fatal(err)
		}
		info, err := manager.Info(args[1])
		if err != nil {
			fatal(err)
		}
		fmt.Println(info)
	case "install":
		if len(args) < 2 {
			fatal(fmt.Errorf("install command expects a package name"))
		}
		manager, err := pkgmgr.New(conf)
		if err != nil {
			fatal(err)
		}
		if err := manager.Update(ctx); err != nil {
			fatal(err)
		}
		dest, err := manager.Install(ctx, args[1])
		if err != nil {
			fatal(err)
		}
		fmt.Println(dest)
	default:
		flag.Usage()
		os.Exit(1)
	}
}

func defaultConfig() string {
	if env := os.Getenv("OPKG_CONF"); env != "" {
		return env
	}
	return "/etc/opkg/opkg.conf"
}

func printVersion() {
	ts := buildTime
	if ts == "" {
		ts = time.Now().UTC().Format(time.RFC3339)
	}
	fmt.Printf("opkg-go %s (%s)\n", version, ts)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
