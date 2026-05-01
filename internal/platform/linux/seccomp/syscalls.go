// internal/platform/linux/seccomp/syscalls.go
//go:build linux

// Package seccomp — syscall allowlist for the renderer process.
// Lists the syscalls a browser renderer legitimately needs.
// Everything not on this list will be blocked by the seccomp filter
// in Phase 2.
package seccomp

// AllowedSyscalls is the set of syscall names permitted for the renderer.
// Source: Chromium's seccomp policy and standard browser requirements.
var AllowedSyscalls = []string{
	"read", "write", "open", "close", "stat", "fstat", "lstat",
	"poll", "lseek", "mmap", "mprotect", "munmap", "brk",
	"rt_sigaction", "rt_sigprocmask", "rt_sigreturn",
	"ioctl", "pread64", "pwrite64", "readv", "writev",
	"access", "pipe", "select", "sched_yield", "mremap",
	"madvise", "shmget", "shmat", "shmctl", "dup", "dup2",
	"pause", "nanosleep", "getitimer", "alarm", "setitimer",
	"getpid", "sendfile", "socket", "connect", "accept",
	"sendto", "recvfrom", "sendmsg", "recvmsg", "shutdown",
	"bind", "listen", "getsockname", "getpeername", "socketpair",
	"setsockopt", "getsockopt", "clone", "fork", "vfork",
	"execve", "exit", "wait4", "kill", "uname", "fcntl",
	"flock", "fsync", "fdatasync", "truncate", "ftruncate",
	"getdents", "getcwd", "chdir", "rename", "mkdir", "rmdir",
	"creat", "link", "unlink", "symlink", "readlink", "chmod",
	"fchmod", "chown", "fchown", "lchown", "umask",
	"gettimeofday", "getrlimit", "getrusage", "sysinfo", "times",
	"getuid", "syslog", "getgid", "setuid", "setgid",
	"geteuid", "getegid", "getppid", "getpgrp", "setsid",
	"getgroups", "setgroups", "futex", "sched_getaffinity",
	"set_thread_area", "get_thread_area", "arch_prctl",
	"set_tid_address", "restart_syscall", "exit_group",
	"epoll_wait", "epoll_ctl", "tgkill", "waitid",
	"openat", "mkdirat", "newfstatat", "unlinkat", "renameat",
	"linkat", "symlinkat", "readlinkat", "fchmodat", "faccessat",
	"pselect6", "ppoll", "splice", "tee", "sync_file_range",
	"vmsplice", "move_pages", "epoll_pwait", "signalfd",
	"timerfd_create", "eventfd", "fallocate", "timerfd_settime",
	"timerfd_gettime", "accept4", "signalfd4", "eventfd2",
	"epoll_create1", "dup3", "pipe2", "inotify_init1",
	"preadv", "pwritev", "recvmmsg", "prlimit64",
	"sendmmsg", "getrandom", "memfd_create",
}

// BlockedSyscalls is the set of syscall names explicitly denied.
var BlockedSyscalls = []string{
	"ptrace",            // debugging / tracing other processes
	"process_vm_readv",  // cross-process memory read
	"process_vm_writev", // cross-process memory write
	"kexec_load",        // load a new kernel
	"kexec_file_load",
	"init_module", // load a kernel module
	"finit_module",
	"delete_module", // remove a kernel module
	"mount",         // mount filesystems
	"umount2",
	"pivot_root", // change filesystem root
	"chroot",
	"swapon",
	"swapoff",
	"reboot",
	"acct",
	"sethostname",
	"setdomainname",
	"ioperm",
	"iopl",
	"create_module",
	"query_module",
	"nfsservctl",
	"get_kernel_syms",
	"uselib",
}
