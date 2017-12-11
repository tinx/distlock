# distlock

distlock is a command line utility for distributed locking, such that
no two systems can acquire the same lock in the same locking domain
at the same time. distlock is written in Go and uses
[*etcd*](https://github.com/coreos/etcd) as a backend.

## Why?

distlock was developed as an orchestration tool for cluster management.
The configuration management tool would call distlock to acquire a
lock before performing any state changing updates on cluster nodes.
In Puppet and Salt this would have the desired effect of failing the
run while another node was being worked on, so that subsequent runs
would try to perform the operation again. Hence, only one cluster
node would be updated at the same time.

distlock was also started so that I could learn Go while doing something
useful at the same time.

## Usage

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
# distlock --lock-name=percona_prod --lock --reason="Upgrade RPMs"
# yum -y upgrade
# distlock --lock-name=percona_prod --unlock
```

Note that, if this was a script, the unlock line might never be called
if `yum upgrade` failed for some reason, and the lock would never be
released. This *can* be an advantage if you want to avoid other cluster
members to even attempt this step before someone has manually checked
up on this failing script bevor allowing others to continue.

## Installation

## Building from Source

## Testing

## Contributing

