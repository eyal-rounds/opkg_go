package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/oe-mirrors/opkg_go/internal/format"
	"github.com/oe-mirrors/opkg_go/internal/logging"
	"github.com/oe-mirrors/opkg_go/internal/pkgmgr"
	"github.com/oe-mirrors/opkg_go/internal/version"
)

var (
	buildVersion = "dev"
	buildTime    = ""
)

func main() {
	var conf string
	flag.StringVar(&conf, "conf", defaultConfig(), "Path to opkg.conf")
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		usage()
		os.Exit(1)
	}

	logging.Debugf("main: command %s invoked with %d args", args[0], len(args)-1)

	ctx := context.Background()
	cmd := args[0]
	rest := args[1:]

	switch cmd {
	case "version", "--version", "-V":
		printVersion()
		return
	case "update":
		manager := mustManager(conf)
		if err := manager.Update(ctx); err != nil {
			fatal(err)
		}
		fmt.Println("Package lists updated.")
	case "clean":
		manager := mustManager(conf)
		if err := manager.Clean(); err != nil {
			fatal(err)
		}
	case "install":
		runInstall(ctx, conf, rest)
	case "download":
		runDownload(ctx, conf, rest)
	case "upgrade":
		runUpgrade(ctx, conf, rest)
	case "list":
		runList(ctx, conf, rest, false)
	case "list-installed":
		runList(ctx, conf, rest, true)
	case "list-upgradable":
		runListUpgradable(ctx, conf, rest)
	case "info":
		runInfo(ctx, conf, rest)
	case "status":
		runStatus(conf, rest)
	case "find":
		runFind(ctx, conf, rest)
	case "compare-versions":
		runCompareVersions(rest)
	case "print-architecture":
		runPrintArchitecture(conf)
	case "depends":
		runDepends(ctx, conf, rest)
	case "whatdepends":
		runReverse(ctx, conf, rest, "whatdepends", pkgmgr.ReverseDependencyQuery{Field: "Depends"})
	case "whatdependsrec":
		runReverse(ctx, conf, rest, "whatdependsrec", pkgmgr.ReverseDependencyQuery{Field: "Depends", Recursive: true})
	case "whatrecommends":
		runReverse(ctx, conf, rest, "whatrecommends", pkgmgr.ReverseDependencyQuery{Field: "Recommends"})
	case "whatsuggests":
		runReverse(ctx, conf, rest, "whatsuggests", pkgmgr.ReverseDependencyQuery{Field: "Suggests"})
	case "whatprovides":
		runReverse(ctx, conf, rest, "whatprovides", pkgmgr.ReverseDependencyQuery{Field: "Provides"})
	case "whatconflicts":
		runReverse(ctx, conf, rest, "whatconflicts", pkgmgr.ReverseDependencyQuery{Field: "Conflicts"})
	case "whatreplaces":
		runReverse(ctx, conf, rest, "whatreplaces", pkgmgr.ReverseDependencyQuery{Field: "Replaces"})
	default:
		usage()
		os.Exit(1)
	}
}

func runInstall(ctx context.Context, conf string, args []string) {
	if len(args) == 0 {
		fatal(fmt.Errorf("install command expects at least one package name"))
	}
	manager := mustManager(conf)
	if err := manager.Update(ctx); err != nil {
		fatal(err)
	}
	for _, name := range args {
		dest, err := manager.Install(ctx, name)
		if err != nil {
			fatal(err)
		}
		fmt.Printf("%s -> %s\n", name, dest)
	}
}

func runDownload(ctx context.Context, conf string, args []string) {
	if len(args) == 0 {
		fatal(fmt.Errorf("download command expects a package name"))
	}
	manager := mustManager(conf)
	if err := manager.Update(ctx); err != nil {
		fatal(err)
	}
	for _, name := range args {
		dest, err := manager.Download(ctx, name)
		if err != nil {
			fatal(err)
		}
		fmt.Printf("%s -> %s\n", name, dest)
	}
}

func runUpgrade(ctx context.Context, conf string, args []string) {
	manager := mustManager(conf)
	if err := manager.Update(ctx); err != nil {
		fatal(err)
	}
	results, err := manager.Upgrade(ctx, args)
	if err != nil {
		fatal(err)
	}
	if len(results) == 0 {
		fmt.Println("No packages to upgrade.")
		return
	}
	for _, res := range results {
		fmt.Printf("%s: %s -> %s (%s)\n", res.Upgrade.Name, res.Upgrade.Installed, res.Upgrade.Available, res.Destination)
	}
}

func runList(ctx context.Context, conf string, args []string, installedOnly bool) {
	manager := mustManager(conf)
	fs := newFlagSet("list")
	short := fs.Bool("short-description", false, "Display only the first line of the description")
	size := fs.Bool("size", false, "Show package size")
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	patterns := fs.Args()
	if !installedOnly {
		if err := manager.Update(ctx); err != nil {
			fatal(err)
		}
	}
	lines, err := manager.ListPackages(pkgmgr.ListOptions{
		InstalledOnly:    installedOnly,
		Patterns:         patterns,
		ShortDescription: *short,
		IncludeSize:      *size,
	})
	if err != nil {
		fatal(err)
	}
	for _, line := range lines {
		fmt.Println(line)
	}
}

func runListUpgradable(ctx context.Context, conf string, args []string) {
	manager := mustManager(conf)
	fs := newFlagSet("list-upgradable")
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	if err := manager.Update(ctx); err != nil {
		fatal(err)
	}
	candidates, err := manager.ListUpgradable(fs.Args())
	if err != nil {
		fatal(err)
	}
	if len(candidates) == 0 {
		return
	}
	for _, c := range candidates {
		fmt.Printf("%s - %s -> %s %s\n", c.Name, c.Installed, c.Available, c.Description)
	}
}

func runInfo(ctx context.Context, conf string, args []string) {
	manager := mustManager(conf)
	fs := newFlagSet("info")
	fieldsFlag := fs.String("fields", "", "Comma separated list of fields to display")
	short := fs.Bool("short-description", false, "Display only the first line of the description")
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	if err := manager.Update(ctx); err != nil {
		fatal(err)
	}
	patterns := fs.Args()
	if len(patterns) == 0 {
		patterns = []string{"*"}
	}
	paragraphs, err := manager.InfoParagraphs(patterns)
	if err != nil {
		fatal(err)
	}
	fields := splitFields(*fieldsFlag)
	for i, p := range paragraphs {
		if i > 0 {
			fmt.Println()
		}
		fmt.Println(formatParagraph(p, fields, *short))
	}
}

func runStatus(conf string, args []string) {
	manager := mustManager(conf)
	fs := newFlagSet("status")
	fieldsFlag := fs.String("fields", "", "Comma separated list of fields to display")
	short := fs.Bool("short-description", false, "Display only the first line of the description")
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	patterns := fs.Args()
	paragraphs := manager.GlobStatus(patterns)
	fields := splitFields(*fieldsFlag)
	for i, entry := range paragraphs {
		if i > 0 {
			fmt.Println()
		}
		fmt.Println(formatParagraph(entry, fields, *short))
	}
}

func runFind(ctx context.Context, conf string, args []string) {
	if len(args) == 0 {
		fatal(fmt.Errorf("find command expects a pattern"))
	}
	manager := mustManager(conf)
	if err := manager.Update(ctx); err != nil {
		fatal(err)
	}
	matches, err := manager.FindPackages(strings.Join(args, " "))
	if err != nil {
		fatal(err)
	}
	for _, pkg := range matches {
		desc := pkg.Description
		if idx := strings.IndexByte(desc, '\n'); idx >= 0 {
			desc = desc[:idx]
		}
		fmt.Printf("%s - %s\n", pkg.Name, desc)
	}
}

func runCompareVersions(args []string) {
	if len(args) != 3 {
		fatal(fmt.Errorf("compare-versions expects <v1> <op> <v2>"))
	}
	ok, err := version.CompareOp(args[0], args[1], args[2])
	if err != nil {
		fatal(err)
	}
	if ok {
		fmt.Println("true")
	} else {
		fmt.Println("false")
	}
}

func runDepends(ctx context.Context, conf string, args []string) {
	includeAll, patterns := parseIncludeAll("depends", args)
	if len(patterns) == 0 {
		fatal(fmt.Errorf("depends expects at least one package name"))
	}
	manager := mustManager(conf)
	if err := manager.Update(ctx); err != nil {
		fatal(err)
	}
	paragraphs, err := manager.InfoParagraphs(patterns)
	if err != nil {
		fatal(err)
	}
	if len(paragraphs) == 0 {
		return
	}
	printed := false
	for _, p := range paragraphs {
		name := p.Value("Package")
		if !includeAll && !manager.Status().Installed(name) {
			continue
		}
		if name == "" {
			continue
		}
		if printed {
			fmt.Println()
		}
		fmt.Printf("Package: %s\n", name)
		for _, field := range []string{"Depends", "Pre-Depends", "Recommends", "Suggests", "Provides", "Conflicts", "Replaces"} {
			if value := p.Value(field); value != "" {
				fmt.Printf("  %s: %s\n", field, value)
			}
		}
		printed = true
	}
}

func runPrintArchitecture(conf string) {
	manager := mustManager(conf)
	arches := manager.Architectures()
	for _, arch := range arches {
		if arch.Priority != 0 {
			fmt.Printf("%s %d\n", arch.Name, arch.Priority)
			continue
		}
		fmt.Println(arch.Name)
	}
}

func runReverse(ctx context.Context, conf string, args []string, name string, query pkgmgr.ReverseDependencyQuery) {
	includeAll, patterns := parseIncludeAll(name, args)
	query.IncludeAll = includeAll
	query.Patterns = patterns
	manager := mustManager(conf)
	if err := manager.Update(ctx); err != nil {
		fatal(err)
	}
	matches, err := manager.ReverseDependencies(query)
	if err != nil {
		fatal(err)
	}
	for _, name := range matches {
		fmt.Println(name)
	}
}

func parseIncludeAll(name string, args []string) (bool, []string) {
	fs := newFlagSet(name)
	all := fs.Bool("A", false, "Query all packages, not just installed ones")
	fs.BoolVar(all, "all", false, "Query all packages, not just installed ones")
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	return *all, fs.Args()
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

func splitFields(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func formatParagraph(p format.Paragraph, fields []string, short bool) string {
	type kv struct {
		key, value string
	}
	var pairs []kv
	if len(fields) == 0 {
		for _, key := range p.Keys() {
			value := p.Fields[key]
			if value == "" {
				continue
			}
			if short && strings.EqualFold(key, "Description") {
				value = trimDescription(value)
			}
			pairs = append(pairs, kv{key: key, value: value})
		}
	} else {
		for _, field := range fields {
			if key, value, ok := lookupField(p, field); ok {
				if value == "" {
					continue
				}
				if short && strings.EqualFold(key, "Description") {
					value = trimDescription(value)
				}
				pairs = append(pairs, kv{key: key, value: value})
			}
		}
	}
	lines := make([]string, 0, len(pairs))
	for _, entry := range pairs {
		lines = append(lines, fmt.Sprintf("%s: %s", entry.key, strings.ReplaceAll(entry.value, "\n", "\n ")))
	}
	return strings.Join(lines, "\n")
}

func lookupField(p format.Paragraph, field string) (string, string, bool) {
	for key, value := range p.Fields {
		if strings.EqualFold(key, field) {
			return key, value, true
		}
	}
	return "", "", false
}

func trimDescription(text string) string {
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		return text[:idx]
	}
	return text
}

func usage() {
	fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [options...] sub-command [arguments...]\n", os.Args[0])
	fmt.Fprintln(flag.CommandLine.Output(), "\nPackage Manipulation:")
	fmt.Fprintln(flag.CommandLine.Output(), "  update                          Update list of available packages")
	fmt.Fprintln(flag.CommandLine.Output(), "  upgrade [pkgs]                  Upgrade installed packages")
	fmt.Fprintln(flag.CommandLine.Output(), "  install <pkgs>                  Install package(s)")
	fmt.Fprintln(flag.CommandLine.Output(), "  download <pkgs>                 Download package(s) to the cache")
	fmt.Fprintln(flag.CommandLine.Output(), "  clean                           Clean internal cache")
	fmt.Fprintln(flag.CommandLine.Output(), "\nInformational Commands:")
	fmt.Fprintln(flag.CommandLine.Output(), "  list [glob]                     List available packages")
	fmt.Fprintln(flag.CommandLine.Output(), "  list-installed [glob]           List installed packages")
	fmt.Fprintln(flag.CommandLine.Output(), "  list-upgradable [glob]          List installed and upgradable packages")
	fmt.Fprintln(flag.CommandLine.Output(), "  info [pkg|glob]                 Display package metadata")
	fmt.Fprintln(flag.CommandLine.Output(), "  status [pkg|glob]               Display installed package status")
	fmt.Fprintln(flag.CommandLine.Output(), "  find <substring>                Search packages by name or description")
	fmt.Fprintln(flag.CommandLine.Output(), "  depends [-A] [pkg|glob]+        Show package dependencies")
	fmt.Fprintln(flag.CommandLine.Output(), "  whatdepends[-A] [pkg|glob]+     List packages depending on the target")
	fmt.Fprintln(flag.CommandLine.Output(), "  whatdependsrec[-A] [pkg|glob]+  Recursively list dependencies")
	fmt.Fprintln(flag.CommandLine.Output(), "  whatrecommends[-A] [pkg|glob]+  List recommending packages")
	fmt.Fprintln(flag.CommandLine.Output(), "  whatsuggests[-A] [pkg|glob]+    List suggesting packages")
	fmt.Fprintln(flag.CommandLine.Output(), "  whatprovides [-A] [pkg|glob]+   List packages providing the target")
	fmt.Fprintln(flag.CommandLine.Output(), "  whatconflicts[-A] [pkg|glob]+   List conflicting packages")
	fmt.Fprintln(flag.CommandLine.Output(), "  whatreplaces [-A] [pkg|glob]+   List packages that replace the target")
	fmt.Fprintln(flag.CommandLine.Output(), "  compare-versions <v1> <op> <v2> Compare version strings")
	fmt.Fprintln(flag.CommandLine.Output(), "  print-architecture              List configured architectures")
	fmt.Fprintln(flag.CommandLine.Output(), "  version                         Print version information")
	fmt.Fprintln(flag.CommandLine.Output(), "\nOptions:")
	flag.PrintDefaults()
}

func defaultConfig() string {
	if env := os.Getenv("OPKG_CONF"); env != "" {
		return env
	}
	return "/etc/opkg/opkg.conf"
}

func mustManager(conf string) *pkgmgr.Manager {
	manager, err := pkgmgr.New(conf)
	if err != nil {
		fatal(err)
	}
	return manager
}

func printVersion() {
	ts := buildTime
	if ts == "" {
		ts = time.Now().UTC().Format(time.RFC3339)
	}
	logging.Debugf("main: printing version %s built at %s", buildVersion, ts)
	fmt.Printf("opkg-go %s (%s)\n", buildVersion, ts)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
