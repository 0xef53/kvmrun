package main

import (
	"log"
	"os"
	"path/filepath"
	"time"

	qmp "github.com/0xef53/go-qmp/v2"

	"github.com/0xef53/kvmrun/pkg/kvmrun"
)

var (
	Info  *log.Logger
	Error *log.Logger
)

func c_Stop(mon *qmp.Monitor) int {
	Info.Println("stopping the emulation")

	if err := mon.Run(qmp.Command{"stop", nil}, nil); err != nil {
		Error.Println(err)
	}

	return 0
}

func c_Cont(mon *qmp.Monitor) int {
	Info.Println("resuming the emulation")

	if err := mon.Run(qmp.Command{"cont", nil}, nil); err != nil {
		Error.Println(err)
	}

	return 1
}

func c_Exit(mon *qmp.Monitor) int {
	Info.Println("forced shutdown: sending quit signal")

	if err := mon.Run(qmp.Command{"quit", nil}, nil); err != nil {
		Error.Println(err)
	}

	return 0
}

func c_Term(mon *qmp.Monitor) int {
	Info.Println("forced resuming the emulation")

	if err := mon.Run(qmp.Command{"cont", nil}, nil); err != nil {
		Error.Println(err)
	}

	Info.Println("graceful shutdown: sending system_powerdown signal")

	if err := mon.Run(qmp.Command{"system_powerdown", nil}, nil); err != nil {
		Error.Println(err)
	}

	return 0
}

func c_Down(mon *qmp.Monitor) int {
	terminated := make(chan struct{})
	timeout := make(chan struct{})

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-timeout:
				return
			case <-ticker.C:
			}
			if err := mon.Run(qmp.Command{"system_powerdown", nil}, nil); err != nil {
				// It means the socket is closed. That is what we need
				break
			}
		}
		close(terminated)
	}()

	select {
	case <-time.After(time.Second * 30):
		close(timeout)
		Info.Println("timed out: sending quit signal")
		if err := mon.Run(qmp.Command{"quit", nil}, nil); err != nil {
			Error.Println(err)
		}
	case <-terminated:
		Info.Println("has been terminated")
	}

	return 0
}

func init() {
	os.Stderr = os.Stdout

	var cname string

	switch filepath.Base(os.Args[0]) {
	case "t":
		cname = "c_Term"
	case "d":
		cname = "c_Down"
	case "x":
		cname = "c_Exit"
	case "p":
		cname = "c_Stop"
	case "c":
		cname = "c_Cont"
	default:
		cname = "control"
	}

	Info = log.New(os.Stdout, cname+": info: ", 0)
	Error = log.New(os.Stdout, cname+": error: ", 0)
}

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		Error.Fatalln(err)
	}

	vmname := filepath.Base(cwd)

	mon, err := qmp.NewMonitor(filepath.Join(kvmrun.QMPMONDIR, vmname+".qmp1"), time.Second*30)
	if err != nil {
		Error.Fatalln(err)
	}
	defer mon.Close()

	var f func(*qmp.Monitor) int

	progname := filepath.Base(os.Args[0])
	switch progname {
	case "t":
		f = c_Term
	case "d":
		f = c_Down
	case "x":
		f = c_Exit
	case "p":
		f = c_Stop
	case "c":
		f = c_Cont
	default:
		Error.Fatalln("Unknown command name:", progname)
	}

	os.Exit(f(mon))
}
