package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func perform_unlock(lock_name string) {
	return
}

func perform_lock(lock_name string, reason string, timeout int) {
	return
}

func perform_command() int {
	args := flag.Args()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "distlock: cmd execution failed: ", err)
		return 2
	}
	if err := cmd.Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			/* exiterr is now type asserted to be type ExitError */
			status, ok := exiterr.Sys().(syscall.WaitStatus)
			if ok {
				/* status is type asserted to be a WaitStatus */
				return status.ExitStatus()
			} else {
				fmt.Println("distlock: unexpected WaitStatus")
				return 2
			}
		} else {
			fmt.Println("distlock: unexpected ExitError")
			return 2
		}
	}
	return 0
}

func main() {
	lock_name := flag.String("lock-name", "",
		"Name of the lock to operate on")
	op_lock := flag.Bool("lock", false, "Acquire lock and exit")
	op_unlock := flag.Bool("unlock", false, "Release lock and exit")
	reason := flag.String("reason", "",
		"Reason why we perform this operation")
	no_wait := flag.Bool("nowait", false, "Fail if the lock is busy")
	timeout := flag.Int("timeout", -1,
		"Max. no. of secs to wait for the lock")

	flag.Parse()

	if *lock_name == "" {
		fmt.Fprintln(os.Stderr, "'lock-name' is a required option.")
		os.Exit(1)
	}
	if *op_lock && *op_unlock {
		fmt.Fprintln(os.Stderr,
			"Can't give both 'lock' and 'unlock' options.")
		os.Exit(1)
	}
	if (*op_lock || *op_unlock) && flag.NArg() > 0 {
		fmt.Fprintln(os.Stderr,
			"Program args given, but would not execute.")
		os.Exit(1)
	}
	if *no_wait {
		if *timeout > 0 {
			fmt.Fprintln(os.Stderr,
				"Conflicting options -nowait and -timeout.")
			os.Exit(1)
		} else {
			*timeout = 0
		}
	}
	if !*op_lock && !*op_unlock && flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "Missing command to protect with lock")
		os.Exit(1)
	}

	if *op_unlock {
		perform_unlock(*lock_name)
		os.Exit(0)
	} else {
		perform_lock(*lock_name, *reason, *timeout)
		if *op_lock {
			/* we're done */
			os.Exit(0)
		} else {
			rc := perform_command()
			perform_unlock(*lock_name)
			os.Exit(rc)
		}
	}
}
