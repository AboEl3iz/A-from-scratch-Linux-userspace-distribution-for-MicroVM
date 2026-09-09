package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"karim-microvm-os/internal/secd"

	"golang.org/x/sys/unix"
)

func main() {
	profileFlag := flag.String("profile", "app-default", "Seccomp BPF profile (app-default, net-service, strict, unrestricted)")
	capsAddFlag := flag.String("caps-add", "", "Comma-separated list of capabilities to retain (e.g. CAP_NET_BIND_SERVICE)")
	capsDropFlag := flag.String("caps-drop", "ALL", "Comma-separated list of capabilities to drop (default: ALL)")
	noNewPrivsFlag := flag.Bool("no-new-privs", true, "Set PR_SET_NO_NEW_PRIVS flag")
	execFlag := flag.String("exec", "", "Path to target binary to execute")

	flag.Parse()

	targetExec := *execFlag
	var targetArgs []string

	// If -- is used, treat positional args as target command and args
	posArgs := flag.Args()
	if targetExec == "" && len(posArgs) > 0 {
		targetExec = posArgs[0]
		targetArgs = posArgs
	} else if targetExec != "" {
		targetArgs = append([]string{targetExec}, posArgs...)
	}

	if targetExec == "" {
		fmt.Fprintf(os.Stderr, "Usage: karim-secd -profile <profile> -exec <binary> [args...]\n")
		os.Exit(1)
	}

	var capsAdd []string
	if *capsAddFlag != "" {
		capsAdd = strings.Split(*capsAddFlag, ",")
	}
	var capsDrop []string
	if *capsDropFlag != "" {
		capsDrop = strings.Split(*capsDropFlag, ",")
	}

	policy := &secd.SecurityPolicy{
		SeccompProfile: *profileFlag,
		CapsAdd:        capsAdd,
		CapsDrop:       capsDrop,
		NoNewPrivs:     *noNewPrivsFlag,
	}

	// Apply hardened security policy
	if err := secd.EnforceSecurityPolicy(policy); err != nil {
		fmt.Fprintf(os.Stderr, "[karim-secd] Security enforcement error: %v\n", err)
		os.Exit(1)
	}

	// Exec target binary replacing current process
	if err := unix.Exec(targetExec, targetArgs, os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "[karim-secd] Failed to exec %s: %v\n", targetExec, err)
		os.Exit(1)
	}
}
