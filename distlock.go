package main

import (
	"flag"
	"os"
	"os/exec"
	"syscall"
	"log"
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
		log.Fatal("distlock: cmd execution failed: ", err)
	}
	if err := cmd.Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			/* exiterr is now type asserted to be type ExitError */
			status, ok := exiterr.Sys().(syscall.WaitStatus)
			if ok {
				/* status is type asserted to be a WaitStatus */
				return status.ExitStatus()
			} else {
				log.Fatal("distlock: unexpected WaitStatus")
			}
		} else {
			log.Fatal("distlock: unexpected ExitError")
		}
	}
	return 0
}

func main() {
	log.SetFlags(0)

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
		log.Fatal("'lock-name' is a required option.")
	}
	if *op_lock && *op_unlock {
		log.Fatal("Can't give both 'lock' and 'unlock' options.")
	}
	if (*op_lock || *op_unlock) && flag.NArg() > 0 {
		log.Fatal("Program args given, but would not execute.")
	}
	if *no_wait {
		if *timeout > 0 {
			log.Fatal("Conflicting options -nowait and -timeout.")
		} else {
			*timeout = 0
		}
	}
	if !*op_lock && !*op_unlock && flag.NArg() == 0 {
		log.Fatal("Missing command to protect with lock")
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
