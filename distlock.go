package main

import (
	"context"
	"flag"
	"fmt"
	v3 "github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/clientv3/concurrency"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var dl_prefix string = "/distlock/"
var dl_internal_lock string = "__internal_lock"
var dl_endpoints string = "http://127.0.0.1:2379"
var dl_maxtime = 5 * time.Second

func init_etcd_client(endpoints []string) (*v3.Client, *concurrency.Session, *concurrency.Mutex) {
	client, err := v3.New(v3.Config{
		Endpoints:   endpoints,
		DialTimeout: dl_maxtime,
	})
	if err != nil {
		log.Fatal("can't connect to etcd: ", err)
	}
	session, err := concurrency.NewSession(client)
	if err != nil {
		log.Fatal("couldn't init session: ", err)
	}
	mutex := concurrency.NewMutex(session, dl_prefix+dl_internal_lock)
	return client, session, mutex
}

func finish_etcd_client(client *v3.Client, session *concurrency.Session) {
	session.Close()
	client.Close()
}

func acquire_state_lock(mutex *concurrency.Mutex) error {
	ctx, cancel := context.WithTimeout(context.Background(), dl_maxtime)
	defer cancel()
	if err := mutex.Lock(ctx); err != nil {
		return err
	}
	return nil
}

func release_state_lock(mutex *concurrency.Mutex) error {
	ctx, cancel := context.WithTimeout(context.Background(), dl_maxtime)
	defer cancel()
	if err := mutex.Unlock(ctx); err != nil {
		return err
	}
	return nil
}

func perform_list(client *v3.Client, mutex *concurrency.Mutex) {
	var ctx context.Context
	var cancel context.CancelFunc

	/* Step 1: get distlock internal state lcok */
	if err := acquire_state_lock(mutex); err != nil {
		log.Fatal("couldn't get state lock: ", err)
	}
	/* Step 2: slurp in all entries */
	ctx, cancel = context.WithTimeout(context.Background(), dl_maxtime)
	resp, err := client.Get(ctx, dl_prefix, v3.WithPrefix(),
		v3.WithSort(v3.SortByKey, v3.SortAscend))
	cancel()
	if err != nil {
		log.Fatal("error retrieving lock list: ", err)
	}
	/* Step 3: unlock distlock internal state lock */
	if err := release_state_lock(mutex); err != nil {
		log.Fatal("error releasing state lock: ", err)
	}
	/* Step 4: print list */
	log.Print("Lock name            Since  Reason")
	for _, ev := range resp.Kvs {
		l := strings.TrimPrefix(string(ev.Key), dl_prefix)
		if strings.Contains(l, "/") {
			continue
		}
		v := strings.SplitN(string(ev.Value), ";", 2)
		if len(v) != 2 {
			continue
		}
		t, err := strconv.ParseInt(v[0], 10, 64)
		if err != nil {
			log.Fatal("internal error: unexpected value format")
		}
		now := time.Now().Unix()
		duration := now - t
		log.Printf("%-20s %2dm%02ds %s", l,
			duration/60, duration%60, v[1])
	}
	return
}

func perform_unlock(client *v3.Client, mutex *concurrency.Mutex, lock_name string) {
	var ctx context.Context
	var cancel context.CancelFunc
	lock_path := dl_prefix + lock_name

	/* Step 1: get distlock internal state lcok */
	if err := acquire_state_lock(mutex); err != nil {
		log.Fatal("couldn't get state lock: ", err)
	}
	/* Step 2: delete the entry */
	ctx, cancel = context.WithTimeout(context.Background(), dl_maxtime)
	if _, err := client.Delete(ctx, lock_path); err != nil {
		log.Fatal("error deleting lock entry: ", err)
	}
	cancel()
	/* Step 3: unlock distlock internal state lock */
	if err := release_state_lock(mutex); err != nil {
		log.Fatal("error releasing state lock: ", err)
	}
	return
}

func perform_lock(client *v3.Client, mutex *concurrency.Mutex, lock_name string, reason string, timeout int) {
	var ctx context.Context
	var cancel context.CancelFunc
	lock_path := dl_prefix + lock_name
	timeout_moment := time.Now().Add(time.Duration(timeout) * time.Second)

	if lock_name == "__internal_lock" {
		log.Fatal("illegal lock name: this name reserved")
	}
again:
	/* Step 1: get distlock internal state lcok */
	if err := acquire_state_lock(mutex); err != nil {
		log.Fatal("couldn't get state lock: ", err)
	}
	/* Step 2: verify that the requested distlock is free */
	ctx, cancel = context.WithTimeout(context.Background(), dl_maxtime)
	resp, err := client.Get(ctx, lock_path)
	cancel()
	if err != nil {
		log.Fatal("can't get lock data: ", err)
	}
	/* Step 3a: if not, release internal state lock, set a watch and wait */
	if len(resp.Kvs) > 0 {
		/* lock entry exists, hence it is locked */
		if timeout == 0 {
			log.Fatal("resource locked.")
		}
		if timeout < 0 {
			ctx, cancel = context.WithCancel(context.Background())
		} else {
			ctx, cancel = context.WithDeadline(context.Background(),
				timeout_moment)
		}
		rch := client.Watch(ctx, lock_path)
		if err := release_state_lock(mutex); err != nil {
			log.Fatal("error releasing state lock: ", err)
		}
		/* wait for events until timeout or DELETE event occur */
		for wresp := range rch {
			if wresp.Canceled {
				log.Fatal("timeout exceeded, giving up")
			}
			for _, ev := range wresp.Events {
				if ev.Type == mvccpb.DELETE {
					cancel()
					goto again
				}
			}
		}
		log.Fatal("timeout exceeded, giving up")
	}
	/* Step 3b: if it is free, lock it */
	entry := fmt.Sprintf("%d;%s", time.Now().Unix(), reason)
	ctx, cancel = context.WithTimeout(context.Background(), dl_maxtime)
	_, err = client.Put(ctx, lock_path, entry)
	if err != nil {
		log.Fatal("error setting lock: ", err)
	}
	cancel()
	/* Step 4: unlock distlock internal state lock */
	if err := release_state_lock(mutex); err != nil {
		log.Fatal("error releasing state lock: ", err)
	}
	return
}

func perform_command() int {
	/* prepare arguments and input and output streams */
	args := flag.Args()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	/* launch command */
	if err := cmd.Start(); err != nil {
		log.Fatal("distlock: cmd execution failed: ", err)
	}
	/* figure out the command's exit code */
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
	/* disable timestamp and other extra data in our output */
	log.SetFlags(0)

	/* parse command line parameters */
	lock_name := flag.String("lock-name", "",
		"Name of the lock to operate on")
	op_lock := flag.Bool("lock", false,
		"Acquire lock and exit")
	op_unlock := flag.Bool("unlock", false,
		"Release lock and exit")
	op_list := flag.Bool("list", false,
		"Print a list of distlocks currently in use and exit")
	reason := flag.String("reason", "",
		"Reason why we perform this operation")
	no_wait := flag.Bool("nowait", false,
		"Fail if the lock is busy")
	timeout := flag.Int("timeout", -1,
		"Max. no. of secs to wait for the lock")
	endpoints := flag.String("endpoints", dl_endpoints,
		"Comma-seperated list of etcd URLs")
	flag.Parse()

	/* verify and post-process command line parameters */
	if !*op_list && *lock_name == "" {
		log.Fatal("'lock-name' is a required option.")
	}
	if strings.Contains(*lock_name, "/") {
		log.Fatal("illegal character in lock name")
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
	if !*op_list && !*op_lock && !*op_unlock && flag.NArg() == 0 {
		log.Fatal("Missing command to protect with lock")
	}
	endpoint_list := strings.Split(*endpoints, ",")

	/* connect to etcd cluster */
	client, session, mutex := init_etcd_client(endpoint_list)
	defer finish_etcd_client(client, session)

	/* ready to go. what are we supposed to do? */
	if *op_list {
		perform_list(client, mutex)
		os.Exit(0)
	} else if *op_unlock {
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
