package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"sync"
	"text/template"

	"github.com/dmgk/getopt"
	"github.com/mitchellh/go-homedir"
)

var usageTmpl = template.Must(template.New("usage").Parse(`
Usage: {{.progname}} [-R path][-qhV] [category/port ...]

Bump port revisions.

Options:
  -R path        ports tree root (default: {{.portsRoot}})
  -q             be quiet
  -h             print help and exit
  -V             print version and exit

Arguments:
  category/port  port origin(s) to bump PORTREVISION of

  Alternatively, pipe a space separated category/port list
  (e.g. from "portgrep -1" to the {{.progname}} standard input.
`[1:]))

var (
	progname  string
	portsRoot = "/usr/ports"
	quiet     bool
	version   = "devel"
)

func showUsage() {
	err := usageTmpl.Execute(os.Stdout, map[string]interface{}{
		"progname":  progname,
		"portsRoot": portsRoot,
	})
	if err != nil {
		panic(fmt.Sprintf("error executing template %q: %s", usageTmpl.Name(), err))
	}
}

func showVersion() {
	fmt.Printf("%s %s\n", progname, version)
}

func errExit(format string, v ...any) {
	fmt.Fprint(os.Stderr, progname, ": ")
	fmt.Fprintf(os.Stderr, format, v...)
	fmt.Fprintln(os.Stderr)
	os.Exit(1)
}

func main() {
	if v, ok := os.LookupEnv("PORTSDIR"); ok && v != "" {
		portsRoot = v
	}

	opts, err := getopt.New("R:qhV")
	if err != nil {
		panic(fmt.Sprintf("error creating options parser: %s", err))
	}
	progname = opts.ProgramName()

	for opts.Scan() {
		opt, err := opts.Option()
		if err != nil {
			errExit(err.Error())
		}

		switch opt.Opt {
		case 'R':
			arg := opt.String()
			if arg != "" {
				portsRoot, err = homedir.Expand(arg)
				if err != nil {
					errExit("error expanding ports root: %s", err.Error())
				}
			} else {
				errExit("ports root cannot be blank")
			}
		case 'q':
			quiet = true
		case 'h':
			showUsage()
			os.Exit(0)
		case 'V':
			showVersion()
			os.Exit(0)
		default:
			panic("unhandled option: -" + string(opt.Opt))
		}
	}

	origchan := make(chan string)
	go processOrigins(origchan)

	origins := opts.Args()
	if len(origins) > 0 {
		// process origins given on the command line
		for _, o := range origins {
			origchan <- o
		}
	} else {
		// no origins were given as arguments, read from stdin
		sc := bufio.NewScanner(os.Stdin)
		sc.Split(bufio.ScanWords)
		for sc.Scan() {
			origchan <- sc.Text()
		}
	}

	close(origchan)
}

type result struct {
	origin string
	err    error
}

func processOrigins(origchan chan string) {
	reschan := make(chan result)
	sem := make(chan int, runtime.NumCPU())

	go func() {
		defer close(reschan)

		var wg sync.WaitGroup
		for origin := range origchan {
			sem <- 1
			wg.Add(1)

			go func(origin string) {
				defer func() {
					<-sem
					wg.Done()
				}()
				reschan <- result{
					origin,
					processPort(filepath.Join(portsRoot, origin, "Makefile")),
				}
			}(origin)
		}
		wg.Wait()
	}()

	for res := range reschan {
		if res.err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: %s\n", progname, res.origin, res.err)
			continue
		}
		if !quiet {
			fmt.Println(res.origin)
		}
	}
}

func processPort(makefilePath string) error {
	f, err := os.OpenFile(makefilePath, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	buf, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	buf, err = bumpPortrevision(buf)
	if err != nil {
		return err
	}

	_, err = f.Seek(0, 0)
	if err != nil {
		return err
	}

	_, err = f.Write(buf)
	return err
}

var (
	distversionRe  = regexp.MustCompile(`((?:\A|\n)\s*DISTVERSION\s*\??=.*(?:\n|\z))`)
	portversionRe  = regexp.MustCompile(`((?:\A|\n)\s*PORTVERSION\s*\??=.*(?:\n|\z))`)
	portrevisionRe = regexp.MustCompile(`((?:\A|\n)\s*PORTREVISION\s*\??=\s*)([^\s]+)(.*(?:\n|\z))`)
)

func bumpPortrevision(buf []byte) ([]byte, error) {
	const rev1 = "${1}PORTREVISION=\t1\n"

	if m := portrevisionRe.FindSubmatch(buf); m != nil {
		rev, err := strconv.ParseUint(string(m[2]), 10, 64)
		if err != nil {
			if err.(*strconv.NumError).Err == strconv.ErrSyntax {
				return nil, errors.New("not a numeric PORTREVISION")
			}
			return nil, err
		}
		buf = portrevisionRe.ReplaceAll(buf, []byte(string(m[1])+strconv.FormatUint(rev+1, 10)+string(m[3])))
	} else if distversionRe.Match(buf) {
		buf = distversionRe.ReplaceAll(buf, []byte(rev1))
	} else if portversionRe.Match(buf) {
		buf = portversionRe.ReplaceAll(buf, []byte(rev1))
	}
	return buf, nil
}
