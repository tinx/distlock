package main

import (
    "flag"
    "fmt"
    "os"
)

func main() {
    lock_name := flag.String("lock-name", "", "Name of the lock to operate on")
    op_lock := flag.Bool("lock", false, "Acquire lock and exit")
    op_unlock := flag.Bool("unlock", false, "Release lock and exit")
    reason := flag.String("reason", "", "Reason why we perform this operation")

    flag.Parse()

    if *op_lock && *op_unlock {
        fmt.Fprintln(os.Stderr, "Can't give both 'lock' and 'unlock' options.")
        os.Exit(1)
    }

    fmt.Println("lock_name:", *lock_name)
    fmt.Println("op_lock:", *op_lock)
    fmt.Println("op_unlock:", *op_unlock)
    fmt.Println("reason:", *reason)
}
