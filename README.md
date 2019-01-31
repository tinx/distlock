# distlock

`distlock` is a command line utility for distributed locking, such that
no two systems can acquire the same lock in the same locking domain
at the same time. distlock is written in Go and uses
[*etcd*](https://github.com/coreos/etcd) as a backend.

## Why?

`distlock` was developed as an orchestration tool for cluster management.
The configuration management tool would call distlock to acquire a
lock before performing any state changing updates on cluster nodes.
In Puppet and Salt this would have the desired effect of failing the
run while another node was being worked on, so that subsequent runs
would try to perform the operation again. Hence, only one cluster
node would be updated at the same time.

`distlock` was also started so that I could learn Go while doing something
useful at the same time.

## Options

| Option      | Description |
| :---        | :--- |
| `lock-name` | Required. Name of the lock to operate on. |
| `lock`      | Acquire the lock, then exit with the lock still acquired. |
| `unlock`    | Release the lock, then exit. |
| `reason`    | Reason why we perform this operation. (ignored for --unlock) |
| `nowait`    | When acquiring a lock, don't wait for it. Instead, fail immediately. (same as "-timeout=0") |
| `timeout`   | Maximum amount of time in seconds to wait for a lock before failing. (default is "-1": wait indefinitely) |
| `endpoints` | Comma-seperated list of etcd endpoint URLs. (default: "http://127.0.0.1:2379") |

## Environment

`distlock` recognizes the following environment vairables:

 - `DISTLOCK_LOCKNAME` is equivalent to `--lock-name`
 - `DISTLOCK_REASON` is equivalent to `--reason`
 - `DISTLOCK_ENDPOINTS` is equivalent to `--endpoints`
 - `DISTLOCK_TIMEOUT` is equivalent to `--timeout`
 - `DISTLOCK_PREFIX` controls which internal prefix `distlock` will use when storing data in etcd. This can be useful if you want multiple 'distlock realms' using the same etcd cluster, for example one for staging and one for testing. The default prefix is `/distlock/`
 - `DISTLOCK_CONFIG` control which config file to read on startup. The default is `/etc/distlock/distlock.yaml`

## Usage Examples

The simple way to use distlock is by telling it which command to
execute, and which lock to obtain first.

```sh
# distlock --lock-name=percona_prod -- yum upgrade
```

This has the advantage that the lock will be released, even in case
of a failure.

As an alternative, distlock can be used to perform the locking steps
individually:

```sh
# distlock --lock-name=percona_prod --lock --reason="Upgrade RPMs on node-3"
# yum -y upgrade
# distlock --lock-name=percona_prod --unlock
```

Note that, if this was a script, the unlock line might never be called
if `yum upgrade` failed for some reason, and the lock would never be
released. This *can* be an advantage if you want to avoid other cluster
members to even attempt this step before someone has manually checked
up on this failing script bevor allowing others to continue.

## Notes

`distlock` does not maintain a concept of lock ownership. Anyone with
write access to the backend etcd can unlock a lock, no matter who
acquired it. So if you wish to release a lock you can just do so
from any node, process or environment:

```sh
node1 $ distlock --lock-name=foo --lock
node2 $ distlock --lock-name=foo --unlock   # works
```

## Installation

## Building from Source

```sh
git clone https://github.com/tinx/distlock
cd distlock
go get ./...
go build
```

## Testing

## Contributing

