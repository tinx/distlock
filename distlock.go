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

var dl_prefix string = "/distlock/"
var dl_internal_lock string = "__internal_lock"

func init_etcd_client() (*v3.Client, *concurrency.Session, *concurrency.Mutex) {
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
	mutex := concurrency.NewMutex(session, dl_prefix + dl_internal_lock)
	return client, session, mutex
}

func finish_etcd_client(client *v3.Client, session *concurrency.Session) {
	session.Close()
	client.Close()
}

func acquire_state_lock(mutex *concurrency.Mutex) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := mutex.Lock(ctx); err != nil {
		return err
	}
	cancel()
	return nil
}

func release_state_lock(mutex *concurrency.Mutex) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := mutex.Unlock(ctx); err != nil {
		return err
	}
	cancel()
	return nil
}

func perform_unlock(client *v3.Client, mutex *concurrency.Mutex, lock_name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := mutex.Unlock(ctx); err != nil {
		log.Fatal("couldn't free lock:", err)
	}
	cancel()
	return
}

func perform_lock(client *v3.Client, mutex *concurrency.Mutex, lock_name string, reason string, timeout int) {
	/* Step 1: get distlock internal state lcok */
	if err := acquire_state_lock(mutex); err != nil {
		log.Fatal("couldn't get state lock:", err)
	}
	/* Step 2: verify that the requested distlock is free */
	/* Step 3a: if not, release internal state lock, set a watch and wait */
	/* Step 3b: if it is free, lock it */
	/* Step 4: store the given reason */
	/* Step 5: unlock distlock internal state lock */
	if err := release_state_lock(mutex); err != nil {
		log.Fatal("error releasing state lock:", err)
	}
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
	op_lock := flag.Bool("lock", false,
		"Acquire lock and exit")
	op_unlock := flag.Bool("unlock", false,
		"Release lock and exit")
	reason := flag.String("reason", "",
		"Reason why we perform this operation")
	no_wait := flag.Bool("nowait", false,
		"Fail if the lock is busy")
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

	client, session, mutex := init_etcd_client()
	defer finish_etcd_client(client, session)

	if *op_unlock {
		perform_unlock(client, mutex, *lock_name)
		os.Exit(0)
	} else {
		perform_lock(client, mutex, *lock_name, *reason, *timeout)
		if *op_lock {
			/* we're done */
			os.Exit(0)
		} else {
			rc := perform_command()
			perform_unlock(client, mutex, *lock_name)
			os.Exit(rc)
		}
	}
}
