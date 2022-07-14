##### Synopsis

```
    brev run-tasks -d
```

##### Description

In order for brev to connect to workspaces, there needs to be background daemons
running to manage some things on your local machines environment. Currently, the
one that is being launched by run-tasks is an ssh config file configuration
daemon that periodically udpates a ssh config file with connection information
in order to access you workspaces.

This command has to be run at every boot, see [Configuring SSH Proxy Daemon at Boot](https://docs.brev.dev/howto/configure-ssh-proxy-daemon-at-boot/) to
configure this command to be run at boot.

This command is set to be deprecated in favor of `brev configure`.

##### Examples

to run tasks in the background

```
$ brev run-tasks -d
PID File: /home/f/.brev/task_daemon.pid
Log File: /home/f/.brev/task_daemon.log
```

to run tasks in the foreground

```
$ brev run-tasks
2022/07/11 15:28:44 creating new ssh config
2022/07/11 15:28:48 creating new ssh config

```

##### See Also
- [Configuring SSH Proxy Daemon at Boot](https://docs.brev.dev/howto/configure-ssh-proxy-daemon-at-boot/)
-TODO brev configure docs
