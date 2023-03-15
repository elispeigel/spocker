package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	// "github.com/elispeigel/spocker/container"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s COMMAND\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if len(flag.Args()) == 0 {
		usage()
		os.Exit(1)
	}

	switch flag.Args()[0] {
	case "run":
		run()
	default:
		usage()
		os.Exit(1)
	}
}

func run() {
	if os.Geteuid() != 0 {
		fmt.Fprintf(os.Stderr, "spocker: need root privileges\n")
		os.Exit(1)
	}

	cmd := exec.Command(flag.Args()[1], flag.Args()[2:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET,
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// if err := container.Run(cmd); err != nil {
	//     fmt.Fprintf(os.Stderr, "spocker: %v\n", err)
	//     os.Exit(1)
	// }
}
