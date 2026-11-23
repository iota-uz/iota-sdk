"""pykernel sandbox: confines the current process before any untrusted code runs.

The bootstrap shim calls apply() as its very first step, before it reads a
single frame from the host, so the boundary is in place before the first
instruction of user code. Everything here is irreversible for the process and
every child it forks:

  1. stray inherited file descriptors are closed (only stdio and the bridge stay);
  2. rlimits (AS, CPU, FSIZE, NPROC, NOFILE) are set soft == hard, so the
     process cannot raise them back and there is no window between spawn and a
     host-side prlimit;
  3. all capabilities are dropped and no_new_privs is set;
  4. Landlock (ABI >= 6) limits the filesystem to read-only runtime paths and a
     read-write session workdir, denies TCP bind/connect, and scopes abstract
     unix sockets and signals to the sandbox;
  5. a seccomp filter refuses socket creation and every address-bearing socket
     call (connect, bind, listen, accept, sendmsg, sendto with an address), so
     a socketpair and the inherited bridge are the only sockets; it also
     refuses ptrace, namespaces, mounts, bpf,
     keyrings, perf, io_uring, cross-process memory access and session/process
     group changes, and kills the process on a foreign syscall ABI.

apply() returns a report of what is really in force, verified by probing the
restriction after it is installed. The host refuses a kernel whose report is
missing a required boundary (fail closed).

Run as a script with --probe it applies the configuration from the
PYKERNEL_SANDBOX environment variable and prints the report as JSON; the Go
isolation.Check uses this to verify a real child before any kernel is created.
"""

import ctypes
import errno
import json
import os
import platform
import resource
import struct
import sys
import sysconfig

BOUNDARIES = (
    "no_new_privs",
    "no_capabilities",
    "landlock_fs",
    "landlock_net",
    "landlock_scope",
    "seccomp",
    "rlimits",
    "nproc",
    "workdir_quota",
)

MIN_LANDLOCK_ABI = 6

# --- Landlock ------------------------------------------------------------

_FS_EXECUTE = 1 << 0
_FS_WRITE_FILE = 1 << 1
_FS_READ_FILE = 1 << 2
_FS_READ_DIR = 1 << 3
_FS_REMOVE_DIR = 1 << 4
_FS_REMOVE_FILE = 1 << 5
_FS_MAKE_CHAR = 1 << 6
_FS_MAKE_DIR = 1 << 7
_FS_MAKE_REG = 1 << 8
_FS_MAKE_SOCK = 1 << 9
_FS_MAKE_FIFO = 1 << 10
_FS_MAKE_BLOCK = 1 << 11
_FS_MAKE_SYM = 1 << 12
_FS_REFER = 1 << 13
_FS_TRUNCATE = 1 << 14
_FS_IOCTL_DEV = 1 << 15

_FILE_ONLY = _FS_EXECUTE | _FS_WRITE_FILE | _FS_READ_FILE | _FS_TRUNCATE | _FS_IOCTL_DEV

_NET_BIND_TCP = 1 << 0
_NET_CONNECT_TCP = 1 << 1

_SCOPE_ABSTRACT_UNIX_SOCKET = 1 << 0
_SCOPE_SIGNAL = 1 << 1

_LANDLOCK_CREATE_RULESET_VERSION = 1 << 0
_LANDLOCK_RULE_PATH_BENEATH = 1

# The Landlock syscalls share numbers across architectures (generic table).
_SYS_LANDLOCK_CREATE_RULESET = 444
_SYS_LANDLOCK_ADD_RULE = 445
_SYS_LANDLOCK_RESTRICT_SELF = 446

# --- prctl / capabilities ------------------------------------------------

_PR_CAPBSET_DROP = 24
_PR_SET_NO_NEW_PRIVS = 38
_PR_GET_NO_NEW_PRIVS = 39
_PR_CAP_AMBIENT = 47
_PR_CAP_AMBIENT_CLEAR_ALL = 4
_LINUX_CAPABILITY_VERSION_3 = 0x20080522

# --- seccomp -------------------------------------------------------------

_SECCOMP_SET_MODE_FILTER = 1
_SECCOMP_FILTER_FLAG_TSYNC = 1
_SECCOMP_RET_KILL_PROCESS = 0x80000000
_SECCOMP_RET_ERRNO = 0x00050000
_SECCOMP_RET_ALLOW = 0x7FFF0000
_SECCOMP_RET_USER_NOTIF = 0x7FC00000
_SECCOMP_FILTER_FLAG_NEW_LISTENER = 1 << 3
_O_CREAT = 0o100

# Calls the host supervisor handles for the workdir quota. "emulate": every
# write-family call, whatever the fd, is performed by the host on a duplicate
# of the caller's open file description, so what is checked is what is
# written (no check-then-continue race, no fd-number trust). "always": entry
# creation, counted. "creat_flag": open/openat counted when the flags argument
# at the given index has O_CREAT. "refuse": allocation paths the supervisor
# does not perform, refused with the given errno (callers such as shutil fall
# back to plain writes).
_QUOTA_CALLS = {
    "x86_64": {
        "emulate": [1, 18, 20, 296, 328, 77],      # write pwrite64 writev pwritev pwritev2 ftruncate
        "always": [85, 437, 83, 258, 133, 259,      # creat openat2 mkdir mkdirat mknod mknodat
                   86, 265, 88, 266],               # link linkat symlink symlinkat
        "creat_flag": [(2, 1), (257, 2)],           # open(flags=arg1) openat(flags=arg2)
        "refuse": {
            "fallocate": (285, 95), "truncate": (76, 1), "io_setup": (206, 1), "io_submit": (209, 1),
            "copy_file_range": (326, 18), "sendfile": (40, 22), "splice": (275, 22),
            "setxattr": (188, 95), "lsetxattr": (189, 95), "fsetxattr": (190, 95), "setxattrat": (463, 95),
        },
        "ioctl": 16,
    },
    "aarch64": {
        "emulate": [64, 68, 66, 70, 287, 46],
        "always": [437, 34, 33, 37, 36],
        "creat_flag": [(56, 2)],
        "refuse": {
            "fallocate": (47, 95), "truncate": (45, 1), "io_setup": (0, 1), "io_submit": (2, 1),
            "copy_file_range": (285, 18), "sendfile": (71, 22), "splice": (76, 22),
            "setxattr": (5, 95), "lsetxattr": (6, 95), "fsetxattr": (7, 95), "setxattrat": (463, 95),
        },
        "ioctl": 29,
    },
}

# ioctl requests a kernel may issue; anything else (FICLONE, filesystem space
# reservation, terminal injection) fails with ENOTTY.
_IOCTL_ALLOW = (
    0x5401,  # TCGETS (isatty)
    0x5413,  # TIOCGWINSZ
    0x541B,  # FIONREAD
    0x5421,  # FIONBIO
    0x5450,  # FIONCLEX
    0x5451,  # FIOCLEX
)

_PR_SET_CHILD_SUBREAPER = 36

_BPF_LD_W_ABS = 0x20
_BPF_JEQ_K = 0x15
_BPF_JGE_K = 0x35
_BPF_JSET_K = 0x45
_BPF_RET_K = 0x06

# seccomp_data: nr (u32) @0, arch (u32) @4, ip (u64) @8, args[6] (u64) @16.
_OFF_NR = 0
_OFF_ARCH = 4
_OFF_ARG0 = 16
_OFF_ARG4_LO = 16 + 8 * 4
_OFF_ARG4_HI = _OFF_ARG4_LO + 4

# Namespace flags refused in clone(2). CLONE_NEWTIME (0x80) overlaps the exit
# signal byte of clone(2) and is only reachable through clone3/unshare, which
# are refused outright.
_CLONE_NS_MASK = 0x00020000 | 0x02000000 | 0x04000000 | 0x08000000 | 0x10000000 | 0x20000000 | 0x40000000

# Syscall numbers refused with EPERM, per architecture. socket(2) is refused
# with EACCES: the only channel to the host is the inherited bridge fd, so the
# process needs no new socket of any family (socketpair stays available for
# in-process event loops).
_ARCHES = {
    "x86_64": {
        "audit": 0xC000003E,
        "x32_bit": 0x40000000,
        "seccomp": 317,
        "capset": 126,
        "capget": 125,
        "socket": 41,
        "sendto": 44,
        "clone": 56,
        "clone3": 435,
        "deny": {
            "ptrace": 101, "process_vm_readv": 310, "process_vm_writev": 311,
            "mount": 165, "umount2": 166, "unshare": 272, "setns": 308,
            "bpf": 321, "keyctl": 250, "add_key": 248, "request_key": 249,
            "perf_event_open": 298, "kexec_load": 246, "kexec_file_load": 320,
            "init_module": 175, "finit_module": 313, "delete_module": 176,
            "pivot_root": 155, "chroot": 161, "swapon": 167, "swapoff": 168,
            "reboot": 169, "open_by_handle_at": 304, "name_to_handle_at": 303,
            "userfaultfd": 323, "fanotify_init": 300,
            "io_uring_setup": 425, "io_uring_enter": 426, "io_uring_register": 427,
            "open_tree": 428, "move_mount": 429, "fsopen": 430, "fsconfig": 431,
            "fsmount": 432, "fspick": 433, "mount_setattr": 442, "pidfd_getfd": 438,
            "kcmp": 312, "acct": 163, "quotactl": 179, "quotactl_fd": 443,
            "settimeofday": 164, "clock_settime": 227, "clock_adjtime": 305,
            "adjtimex": 159, "iopl": 172, "ioperm": 173, "lookup_dcookie": 212,
            "vhangup": 153, "setsid": 112, "setpgid": 109, "syslog": 103,
            "sethostname": 170, "setdomainname": 171, "uselib": 134,
            # Address-bearing socket calls: a socketpair is the only socket a
            # kernel can make, and it must not reach a pathname endpoint.
            "connect": 42, "accept": 43, "bind": 49, "listen": 50, "accept4": 288,
            "sendmsg": 46, "sendmmsg": 307,
        },
    },
    "aarch64": {
        "audit": 0xC00000B7,
        "x32_bit": None,
        "seccomp": 277,
        "capset": 91,
        "capget": 90,
        "socket": 198,
        "sendto": 206,
        "clone": 220,
        "clone3": 435,
        "deny": {
            "ptrace": 117, "process_vm_readv": 270, "process_vm_writev": 271,
            "mount": 40, "umount2": 39, "unshare": 97, "setns": 268,
            "bpf": 280, "keyctl": 219, "add_key": 217, "request_key": 218,
            "perf_event_open": 241, "kexec_load": 104, "kexec_file_load": 294,
            "init_module": 105, "finit_module": 273, "delete_module": 106,
            "pivot_root": 41, "chroot": 51, "swapon": 224, "swapoff": 225,
            "reboot": 142, "open_by_handle_at": 265, "name_to_handle_at": 264,
            "userfaultfd": 282, "fanotify_init": 262,
            "io_uring_setup": 425, "io_uring_enter": 426, "io_uring_register": 427,
            "open_tree": 428, "move_mount": 429, "fsopen": 430, "fsconfig": 431,
            "fsmount": 432, "fspick": 433, "mount_setattr": 442, "pidfd_getfd": 438,
            "kcmp": 272, "acct": 89, "quotactl": 60, "quotactl_fd": 443,
            "settimeofday": 170, "clock_settime": 112, "clock_adjtime": 266,
            "adjtimex": 171, "lookup_dcookie": 18, "vhangup": 58,
            "setsid": 157, "setpgid": 154, "syslog": 116,
            "sethostname": 161, "setdomainname": 162,
            "connect": 203, "accept": 202, "bind": 200, "listen": 201, "accept4": 242,
            "sendmsg": 211, "sendmmsg": 269,
        },
    },
}

_ARCH_ALIASES = {"amd64": "x86_64", "arm64": "aarch64"}

_RLIMITS = (
    ("cpu", resource.RLIMIT_CPU),
    ("as", resource.RLIMIT_AS),
    ("fsize", resource.RLIMIT_FSIZE),
    ("nproc", resource.RLIMIT_NPROC),
    ("nofile", resource.RLIMIT_NOFILE),
)


class _Report:
    def __init__(self):
        self.boundaries = {b: {"enabled": False, "detail": "not applied"} for b in BOUNDARIES}
        self.landlock_abi = 0

    def set(self, name, enabled, detail=""):
        self.boundaries[name] = {"enabled": bool(enabled), "detail": detail}

    def missing(self):
        return [b for b in BOUNDARIES if not self.boundaries[b]["enabled"]]

    def as_dict(self):
        return {
            "boundaries": self.boundaries,
            "landlock_abi": self.landlock_abi,
            "missing": self.missing(),
            "arch": _arch() or platform.machine(),
            "uid": os.getuid(),
        }


def _arch():
    machine = platform.machine().lower()
    machine = _ARCH_ALIASES.get(machine, machine)
    return machine if machine in _ARCHES else None


_libc = None


def _lib():
    global _libc
    if _libc is None:
        _libc = ctypes.CDLL(None, use_errno=True)
        _libc.syscall.restype = ctypes.c_long
        _libc.prctl.restype = ctypes.c_int
    return _libc


def _syscall(nr, *args):
    libc = _lib()
    res = libc.syscall(ctypes.c_long(nr), *[ctypes.c_long(a) for a in args])
    if res < 0:
        err = ctypes.get_errno()
        raise OSError(err, os.strerror(err))
    return res


def _prctl(option, arg2=0, arg3=0, arg4=0, arg5=0):
    libc = _lib()
    res = libc.prctl(
        ctypes.c_int(option), ctypes.c_ulong(arg2), ctypes.c_ulong(arg3),
        ctypes.c_ulong(arg4), ctypes.c_ulong(arg5),
    )
    if res < 0:
        err = ctypes.get_errno()
        raise OSError(err, os.strerror(err))
    return res


# --- Steps ---------------------------------------------------------------

def _close_stray_fds(keep):
    keep = sorted({0, 1, 2} | set(keep))
    lo = 0
    for fd in keep:
        if fd > lo:
            os.closerange(lo, fd)
        lo = fd + 1
    os.closerange(lo, 1 << 20)


def _apply_rlimits(cfg, report):
    limits = cfg.get("rlimits") or {}
    problems = []
    for key, res in _RLIMITS:
        want = int(limits.get(key) or 0)
        if want <= 0:
            problems.append("%s not configured" % key)
            continue
        soft, hard = resource.getrlimit(res)
        if hard != resource.RLIM_INFINITY and hard < want:
            want = hard
        try:
            resource.setrlimit(res, (want, want))
        except (OSError, ValueError) as exc:
            problems.append("%s: %s" % (key, exc))
            continue
        got = resource.getrlimit(res)
        if got != (want, want):
            problems.append("%s: got %r" % (key, got))
    report.set("rlimits", not problems, "; ".join(problems) or "soft == hard, set before user code")
    uid = os.getuid()
    nproc = resource.getrlimit(resource.RLIMIT_NPROC)[1]
    if uid == 0:
        report.set("nproc", False, "RLIMIT_NPROC is not enforced for uid 0; run the kernel as an unprivileged uid")
    elif nproc == resource.RLIM_INFINITY:
        report.set("nproc", False, "RLIMIT_NPROC is unlimited")
    else:
        report.set("nproc", True, "uid %d limited to %d tasks" % (uid, nproc))


def _cap_words(arch):
    header = (ctypes.c_uint32 * 2)(_LINUX_CAPABILITY_VERSION_3, 0)
    data = (ctypes.c_uint32 * 6)()
    _syscall(_ARCHES[arch]["capget"], ctypes.addressof(header), ctypes.addressof(data))
    return list(data)


def _drop_capabilities(arch, report):
    try:
        if any(_cap_words(arch)):
            try:
                _prctl(_PR_CAP_AMBIENT, _PR_CAP_AMBIENT_CLEAR_ALL)
            except OSError:
                pass
            for cap in range(64):
                try:
                    _prctl(_PR_CAPBSET_DROP, cap)
                except OSError:
                    pass
            header = (ctypes.c_uint32 * 2)(_LINUX_CAPABILITY_VERSION_3, 0)
            data = (ctypes.c_uint32 * 6)()
            _syscall(_ARCHES[arch]["capset"], ctypes.addressof(header), ctypes.addressof(data))
        words = _cap_words(arch)
        report.set("no_capabilities", not any(words), "effective/permitted/inheritable empty" if not any(words) else "capabilities remain: %r" % words)
    except OSError as exc:
        report.set("no_capabilities", False, "capget/capset: %s" % exc)


def _no_new_privs(report):
    try:
        _prctl(_PR_SET_NO_NEW_PRIVS, 1)
        ok = _prctl(_PR_GET_NO_NEW_PRIVS) == 1
        report.set("no_new_privs", ok, "set" if ok else "prctl reports 0")
    except OSError as exc:
        report.set("no_new_privs", False, "prctl: %s" % exc)


def _handled_fs(abi):
    mask = (1 << 13) - 1
    if abi >= 2:
        mask |= _FS_REFER
    if abi >= 3:
        mask |= _FS_TRUNCATE
    if abi >= 5:
        mask |= _FS_IOCTL_DEV
    return mask


def _runtime_paths():
    """Read-only paths the interpreter itself needs after the restriction:
    the stdlib, site-packages and the interpreter binary."""
    paths = set()
    for p in sys.path:
        if p:
            paths.add(p)
    for key in ("stdlib", "platstdlib", "purelib", "platlib"):
        try:
            p = sysconfig.get_path(key)
        except KeyError:
            p = None
        if p:
            paths.add(p)
    paths.add(os.path.dirname(os.__file__))
    paths.add(os.path.realpath(sys.executable))
    here = os.path.dirname(os.path.abspath(__file__))
    paths.add(here)
    return paths


def _within(path, root):
    path = os.path.realpath(path)
    root = os.path.realpath(root)
    return path == root or path.startswith(root.rstrip("/") + "/")


def _add_rule(ruleset_fd, path, access, handled):
    try:
        fd = os.open(path, os.O_PATH | os.O_CLOEXEC)
    except OSError:
        return False
    try:
        if not os.path.isdir(path):
            access &= _FILE_ONLY
        access &= handled
        if not access:
            return False
        attr = ctypes.create_string_buffer(struct.pack("=Qi", access, fd), 12)
        _syscall(_SYS_LANDLOCK_ADD_RULE, ruleset_fd, _LANDLOCK_RULE_PATH_BENEATH, ctypes.addressof(attr), 0)
        return True
    finally:
        os.close(fd)


def _landlock(cfg, report):
    try:
        abi = _syscall(_SYS_LANDLOCK_CREATE_RULESET, 0, 0, _LANDLOCK_CREATE_RULESET_VERSION)
    except OSError as exc:
        detail = "landlock unavailable: %s" % exc
        for b in ("landlock_fs", "landlock_net", "landlock_scope"):
            report.set(b, False, detail)
        return
    report.landlock_abi = abi
    min_abi = int(cfg.get("min_landlock_abi") or MIN_LANDLOCK_ABI)
    if abi < min_abi:
        detail = "landlock ABI %d < required %d" % (abi, min_abi)
        for b in ("landlock_fs", "landlock_net", "landlock_scope"):
            report.set(b, False, detail)
        return

    workdir = cfg.get("workdir") or ""
    if not workdir or not os.path.isdir(workdir):
        for b in ("landlock_fs", "landlock_net", "landlock_scope"):
            report.set(b, False, "workdir %r is not a directory" % workdir)
        return

    handled_fs = _handled_fs(abi)
    handled_net = _NET_BIND_TCP | _NET_CONNECT_TCP
    scoped = _SCOPE_ABSTRACT_UNIX_SOCKET | _SCOPE_SIGNAL
    attr = (ctypes.c_uint64 * 3)(handled_fs, handled_net, scoped)
    ruleset_fd = _syscall(_SYS_LANDLOCK_CREATE_RULESET, ctypes.addressof(attr), ctypes.sizeof(attr), 0)
    try:
        read = _FS_READ_FILE | _FS_READ_DIR
        read_exec = read | _FS_EXECUTE
        # REFER (link/rename across directories) must stay limited to the
        # workdir: with it on a read path, the kernel could hard-link host
        # files into its workdir, which the host then hands to the kernel uid.
        assert not (read_exec & _FS_REFER)
        write = handled_fs & ~(_FS_EXECUTE | _FS_MAKE_CHAR | _FS_MAKE_BLOCK | _FS_MAKE_SOCK | _FS_IOCTL_DEV)
        rules = [(p, read_exec) for p in sorted(_runtime_paths()) if os.path.isabs(p) and not _within(p, workdir)]
        rules += [(p, read) for p in cfg.get("read_paths") or [] if p and os.path.isabs(p)]
        rules += [(p, read_exec) for p in cfg.get("exec_paths") or [] if p and os.path.isabs(p)]
        for p, _ in rules:
            if os.path.exists(p) and _within(workdir, p):
                # A read rule above the workdir would also expose every
                # sibling session's workdir: refuse the configuration.
                raise OSError(errno.EINVAL, "read path %s contains the workdir" % p)
        for p, access in rules:
            _add_rule(ruleset_fd, p, access, handled_fs)
        _add_rule(ruleset_fd, "/dev/null", _FS_READ_FILE | _FS_WRITE_FILE | _FS_TRUNCATE, handled_fs)
        for dev in ("/dev/zero", "/dev/random", "/dev/urandom"):
            _add_rule(ruleset_fd, dev, _FS_READ_FILE, handled_fs)
        if not _add_rule(ruleset_fd, workdir, write, handled_fs):
            raise OSError(errno.EINVAL, "cannot add the workdir rule")
        _syscall(_SYS_LANDLOCK_RESTRICT_SELF, ruleset_fd, 0)
    except OSError as exc:
        for b in ("landlock_fs", "landlock_net", "landlock_scope"):
            report.set(b, False, "landlock: %s" % exc)
        return
    finally:
        os.close(ruleset_fd)
    report.set("landlock_fs", True, "ABI %d: read-only runtime, read-write workdir" % abi)
    report.set("landlock_net", True, "TCP bind/connect denied")
    report.set("landlock_scope", True, "abstract unix sockets and signals scoped to the sandbox")


def _seccomp_program(arch):
    spec = _ARCHES[arch]
    eperm = _SECCOMP_RET_ERRNO | errno.EPERM
    prog = [
        (_BPF_LD_W_ABS, 0, 0, _OFF_ARCH),
        (_BPF_JEQ_K, 1, 0, spec["audit"]),
        (_BPF_RET_K, 0, 0, _SECCOMP_RET_KILL_PROCESS),
        (_BPF_LD_W_ABS, 0, 0, _OFF_NR),
    ]
    if spec["x32_bit"]:
        prog += [
            (_BPF_JGE_K, 0, 1, spec["x32_bit"]),
            (_BPF_RET_K, 0, 0, _SECCOMP_RET_KILL_PROCESS),
        ]
    rules = [(nr, eperm) for nr in sorted(set(spec["deny"].values()))]
    rules.append((spec["socket"], _SECCOMP_RET_ERRNO | errno.EACCES))
    for nr, err in _QUOTA_CALLS[arch]["refuse"].values():
        rules.append((nr, _SECCOMP_RET_ERRNO | err))
    rules.append((spec["clone3"], _SECCOMP_RET_ERRNO | errno.ENOSYS))
    for nr, action in rules:
        prog += [
            (_BPF_JEQ_K, 0, 1, nr),
            (_BPF_RET_K, 0, 0, action),
        ]
    allowed = _IOCTL_ALLOW
    prog.append((_BPF_JEQ_K, 0, len(allowed) + 3, _QUOTA_CALLS[arch]["ioctl"]))
    prog.append((_BPF_LD_W_ABS, 0, 0, _OFF_ARG0 + 8))
    for j, req in enumerate(allowed):
        prog.append((_BPF_JEQ_K, len(allowed) - j, 0, req))
    prog += [
        (_BPF_RET_K, 0, 0, _SECCOMP_RET_ERRNO | errno.ENOTTY),
        (_BPF_RET_K, 0, 0, _SECCOMP_RET_ALLOW),
    ]
    # sendto(2) is send(2) when its address (arg 4) is NULL; with an address
    # it would reach a pathname datagram socket, so it is refused.
    prog += [
        (_BPF_JEQ_K, 0, 6, spec["sendto"]),
        (_BPF_LD_W_ABS, 0, 0, _OFF_ARG4_LO),
        (_BPF_JEQ_K, 0, 2, 0),
        (_BPF_LD_W_ABS, 0, 0, _OFF_ARG4_HI),
        (_BPF_JEQ_K, 1, 0, 0),
        (_BPF_RET_K, 0, 0, eperm),
        (_BPF_RET_K, 0, 0, _SECCOMP_RET_ALLOW),
    ]
    prog += [
        (_BPF_JEQ_K, 0, 4, spec["clone"]),
        (_BPF_LD_W_ABS, 0, 0, _OFF_ARG0),
        (_BPF_JSET_K, 0, 1, _CLONE_NS_MASK),
        (_BPF_RET_K, 0, 0, eperm),
        (_BPF_RET_K, 0, 0, _SECCOMP_RET_ALLOW),
        (_BPF_RET_K, 0, 0, _SECCOMP_RET_ALLOW),
    ]
    return prog


class _SockFilter(ctypes.Structure):
    _fields_ = [("code", ctypes.c_uint16), ("jt", ctypes.c_uint8), ("jf", ctypes.c_uint8), ("k", ctypes.c_uint32)]


class _SockFprog(ctypes.Structure):
    _fields_ = [("len", ctypes.c_ushort), ("filter", ctypes.POINTER(_SockFilter))]


def _seccomp(arch, report):
    prog = _seccomp_program(arch)
    filters = (_SockFilter * len(prog))(*[_SockFilter(*ins) for ins in prog])
    fprog = _SockFprog(len(prog), filters)
    try:
        _syscall(_ARCHES[arch]["seccomp"], _SECCOMP_SET_MODE_FILTER, _SECCOMP_FILTER_FLAG_TSYNC, ctypes.addressof(fprog))
    except OSError as exc:
        report.set("seccomp", False, "seccomp: %s" % exc)
        return
    report.set("seccomp", True, "%d-instruction filter installed" % len(prog))


def _quota_program(arch):
    spec = _ARCHES[arch]
    calls = _QUOTA_CALLS[arch]
    prog = [
        (_BPF_LD_W_ABS, 0, 0, _OFF_ARCH),
        (_BPF_JEQ_K, 1, 0, spec["audit"]),
        (_BPF_RET_K, 0, 0, _SECCOMP_RET_ALLOW),  # the main filter kills foreign ABIs
        (_BPF_LD_W_ABS, 0, 0, _OFF_NR),
    ]
    for nr in calls["emulate"] + calls["always"]:
        prog += [
            (_BPF_JEQ_K, 0, 1, nr),
            (_BPF_RET_K, 0, 0, _SECCOMP_RET_USER_NOTIF),
        ]
    for nr, arg in calls["creat_flag"]:
        prog += [
            (_BPF_JEQ_K, 0, 4, nr),
            (_BPF_LD_W_ABS, 0, 0, _OFF_ARG0 + 8 * arg),
            (_BPF_JSET_K, 0, 1, _O_CREAT),
            (_BPF_RET_K, 0, 0, _SECCOMP_RET_USER_NOTIF),
            (_BPF_RET_K, 0, 0, _SECCOMP_RET_ALLOW),
        ]
    prog.append((_BPF_RET_K, 0, 0, _SECCOMP_RET_ALLOW))
    return prog


def _quota_supervision(arch, report, bridge_fd):
    """Install the user-notification filter and hand its listener to the host
    over the bridge (one byte "N" with SCM_RIGHTS). Must run before the main
    filter, which refuses sendmsg. Without a bridge (probe) the listener is
    only created and closed."""
    prog = _quota_program(arch)
    filters = (_SockFilter * len(prog))(*[_SockFilter(*ins) for ins in prog])
    fprog = _SockFprog(len(prog), filters)
    if bridge_fd is None:
        # Probe: with no supervisor every write would fail, so install the
        # filter in a short-lived child that exits without writing.
        pid = os.fork()
        if pid == 0:
            try:
                _syscall(_ARCHES[arch]["seccomp"], _SECCOMP_SET_MODE_FILTER, _SECCOMP_FILTER_FLAG_NEW_LISTENER, ctypes.addressof(fprog))
                os._exit(0)
            except BaseException:
                os._exit(1)
        _, status = os.waitpid(pid, 0)
        ok = os.WIFEXITED(status) and os.WEXITSTATUS(status) == 0
        report.set("workdir_quota", ok, "seccomp user notification available; the Manager verifies enforcement with a test kernel"
                   if ok else "seccomp user notification unavailable")
        return False
    try:
        lfd = _syscall(_ARCHES[arch]["seccomp"], _SECCOMP_SET_MODE_FILTER, _SECCOMP_FILTER_FLAG_NEW_LISTENER, ctypes.addressof(fprog))
    except OSError as exc:
        report.set("workdir_quota", False, "seccomp user notification: %s" % exc)
        return False
    try:
        import socket

        s = socket.socket(fileno=os.dup(bridge_fd))
        try:
            socket.send_fds(s, [b"N"], [lfd])
            global _HANDOFF_SENT
            _HANDOFF_SENT = True
        finally:
            s.close()
        report.set("workdir_quota", True, "file writes and entry creation are approved by the host")
        return True
    except OSError as exc:
        report.set("workdir_quota", False, "hand the listener to the host: %s" % exc)
        return False
    finally:
        os.close(lfd)


# --- Verification --------------------------------------------------------

def _expect_denied(fn):
    try:
        fn()
    except PermissionError:
        return True
    except OSError as exc:
        return exc.errno in (errno.EACCES, errno.EPERM)
    return False


def _verify(arch, report, root_listable):
    import socket

    if report.boundaries["landlock_fs"]["enabled"] and root_listable:
        if not _expect_denied(lambda: os.listdir("/")):
            report.set("landlock_fs", False, "listing / still succeeds after the restriction")
    if report.boundaries["landlock_scope"]["enabled"]:
        ppid = os.getppid()
        if ppid > 1 and not _expect_denied(lambda: os.kill(ppid, 0)):
            report.set("landlock_scope", False, "signalling the parent still succeeds")
    if report.boundaries["seccomp"]["enabled"]:
        if not _expect_denied(lambda: socket.socket(socket.AF_INET, socket.SOCK_STREAM).close()):
            report.set("seccomp", False, "socket(AF_INET) still succeeds")
        elif not _expect_denied(lambda: _syscall(_ARCHES[arch]["deny"]["unshare"], 0)):
            report.set("seccomp", False, "unshare still succeeds")
    if report.boundaries["no_new_privs"]["enabled"]:
        try:
            if _prctl(_PR_GET_NO_NEW_PRIVS) != 1:
                report.set("no_new_privs", False, "prctl reports 0")
        except OSError as exc:
            report.set("no_new_privs", False, "prctl: %s" % exc)


def apply(cfg, keep_fds=(), bridge_fd=None):
    """Confine the current process according to cfg and return the report.

    Call it from the main thread before any other thread starts: Landlock
    applies to the calling thread and the threads and processes it creates
    afterwards.

    With bridge_fd, exactly one byte is written to it before any frame: "N"
    carrying the quota supervisor's listener, or "X" when there is none.
    """
    report = _Report()
    try:
        _apply(cfg, keep_fds, bridge_fd, report)
    finally:
        # Exactly one handshake byte: "N" went out with the listener, or "X".
        if bridge_fd is not None and not _HANDOFF_SENT:
            os.write(bridge_fd, b"X")
    return report


_HANDOFF_SENT = False


def _apply(cfg, keep_fds, bridge_fd, report):
    if not sys.platform.startswith("linux"):
        for b in BOUNDARIES:
            report.set(b, False, "sandbox requires Linux (got %s)" % sys.platform)
        return False
    arch = _arch()
    if arch is None:
        for b in BOUNDARIES:
            report.set(b, False, "unsupported architecture %s" % platform.machine())
        return False
    try:
        _lib()
    except OSError as exc:
        for b in BOUNDARIES:
            report.set(b, False, "libc via ctypes unavailable: %s" % exc)
        return False

    try:
        os.listdir("/")
        root_listable = True
    except OSError:
        root_listable = False

    _close_stray_fds(keep_fds)
    _apply_rlimits(cfg, report)
    _drop_capabilities(arch, report)
    _no_new_privs(report)
    sent = False
    if report.boundaries["no_new_privs"]["enabled"]:
        # Every descendant stays a child of the kernel (and in its CPU
        # accounting) instead of being reparented outside it.
        try:
            _prctl(_PR_SET_CHILD_SUBREAPER, 1)
        except OSError:
            pass
        _landlock(cfg, report)
        sent = _quota_supervision(arch, report, bridge_fd)
        _seccomp(arch, report)
    else:
        for b in ("landlock_fs", "landlock_net", "landlock_scope", "seccomp", "workdir_quota"):
            report.set(b, False, "requires no_new_privs")
    _verify(arch, report, root_listable)
    return sent


def _probe():
    raw = os.environ.get("PYKERNEL_SANDBOX")
    if not raw:
        print(json.dumps({"error": "PYKERNEL_SANDBOX not set"}))
        return 2
    cfg = json.loads(raw)
    report = apply(cfg)
    out = report.as_dict()
    sys.stdout.write(json.dumps(out) + "\n")
    sys.stdout.flush()
    return 0 if not out["missing"] else 3


if __name__ == "__main__":
    if "--probe" in sys.argv[1:]:
        sys.exit(_probe())
    sys.stderr.write("usage: sandbox.py --probe\n")
    sys.exit(2)
