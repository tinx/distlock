package main

import (
	"context"
	"flag"
	v3 "github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/clientv3/concurrency"
	"log"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func init_etcd_client(lock_name string) (*v3.Client, *concurrency.Session, *concurrency.Mutex) {
	client, err := v3.New(v3.Config{
		Endpoints:   []string{"http://127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatal("can't connect to etcd:", err)
	}
	session, err := concurrency.NewSession(client)
	if err != nil {
		log.Fatal("couldn't init session:", err)
	}
	mutex := concurrency.NewMutex(session, "/distlock/"+lock_name)
	return client, session, mutex
}

func finish_etcd_client(client *v3.Client) {
	client.Close()
}

func perform_unlock(mutex *concurrency.Mutex) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := mutex.Unlock(ctx); err != nil {
		log.Fatal("couldn't free lock:", err)
	}
	cancel()
	return
}

func perform_lock(mutex *concurrency.Mutex, reason string, timeout int) {
	var ctx context.Context
	var cancel context.CancelFunc
	if timeout <= 0 {
		ctx, cancel = context.WithCancel(context.Background())
	} else {
		ctx, cancel = context.WithTimeout(context.Background(),
			time.Duration(timeout)*time.Second)
	}
	if err := mutex.Lock(ctx); err != nil {
		log.Fatal("couldn't acquire lock:", err)
	}
	cancel()
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

	client, session, mutex := init_etcd_client(*lock_name)
	defer session.Close()
	defer finish_etcd_client(client)

	if *op_unlock {
		perform_unlock(mutex)
		os.Exit(0)
	} else {
		perform_lock(mutex, *reason, *timeout)
		if *op_lock {
			/* we're done */
			os.Exit(0)
		} else {
			rc := perform_command()
			perform_unlock(mutex)
			os.Exit(rc)
		}
	}
}
