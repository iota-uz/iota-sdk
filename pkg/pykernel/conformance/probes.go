//go:build unix

package conformance

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/iota-uz/iota-sdk/pkg/pykernel"
	"github.com/iota-uz/iota-sdk/pkg/pykernel/isolation"
)

// prelude is prepended to every Python probe. CFG is set before it.
const prelude = `
import ctypes, errno, json, os, platform, socket, sys
_libc = ctypes.CDLL(None, use_errno=True)
_libc.syscall.restype = ctypes.c_long

def report(check, ok, detail=""):
    print("PYKERNEL_PROBE " + json.dumps({"name": check, "ok": bool(ok), "detail": str(detail)[:400]}), flush=True)

def denied(check, fn, also=()):
    try:
        fn()
    except OSError as e:
        report(check, e.errno in (errno.EACCES, errno.EPERM) + tuple(also), "%s: %s" % (type(e).__name__, e))
        return
    except Exception as e:
        report(check, False, "unexpected %s: %s" % (type(e).__name__, e))
        return
    report(check, False, "allowed")

def read(path):
    with open(path, "rb") as f:
        f.read(1)

def write(path):
    with open(path, "wb") as f:
        f.write(b"x")

def connect(family, kind, addr, proto=0):
    s = socket.socket(family, kind, proto)
    try:
        s.settimeout(2)
        if kind == socket.SOCK_DGRAM:
            s.sendto(b"x", addr)
        else:
            s.connect(addr)
    finally:
        s.close()

def checked(res):
    if res == -1:
        e = ctypes.get_errno()
        raise OSError(e, os.strerror(e))
    return res

def raw_syscall(nr, *args):
    return checked(_libc.syscall(ctypes.c_long(nr), *[ctypes.c_long(a) for a in args]))

SYS = {
    "x86_64": {"shmget": 29, "msgget": 68, "semget": 64, "mq_open": 240, "bpf": 321, "keyctl": 250, "perf_event_open": 298, "io_uring_setup": 425, "setns": 308,
               "process_vm_readv": 310, "mount": 165, "capget": 125, "add_key": 248},
    "aarch64": {"shmget": 194, "msgget": 186, "semget": 190, "mq_open": 180, "bpf": 280, "keyctl": 219, "perf_event_open": 241, "io_uring_setup": 425, "setns": 268,
                "process_vm_readv": 270, "mount": 40, "capget": 90, "add_key": 217},
}.get(platform.machine().lower().replace("amd64", "x86_64").replace("arm64", "aarch64"), {})

def addr(s):
    host, port = s.rsplit(":", 1)
    return (host.strip("[]"), int(port))
`

var probes = []probe{
	{"environment", probeEnvironment},
	{"inherited_fds", probeFDs},
	{"read_proc", probeReadProc},
	{"read_home_and_app", probeReadHome},
	{"write_outside_workdir", probeWrite},
	{"exec_from_workdir", probeExecWorkdir},
	{"sibling_workdir", probeSibling},
	{"network_tcp", probeTCP},
	{"network_udp", probeUDP},
	{"network_raw_netlink", probeRaw},
	{"unix_sockets", probeUnix},
	{"ptrace_and_signals", probePtrace},
	{"privileged_syscalls", probeSyscalls},
	{"privileges", probePrivileges},
	{"rlimits", probeRlimits},
	{"memory_exhaustion", probeMemory},
	{"fork_bomb", probeForkBomb},
	{"workdir_quota", probeQuota},
	{"workdir_quota_unlinked_files", probeQuotaUnlinked},
	{"workdir_quota_after_exec", probeQuotaAfterExec},
	{"forged_exec_done", probeForgedDone},
	{"host_ipc", probeHostIPC},
	{"host_process_controls", probeHostProcessControls},
	{"workdir_quota_bypasses", probeQuotaBypasses},
	{"refused_allocators", probeRefusedAllocators},
	{"idle_cpu_via_children", probeIdleForkEvasion},
	{"no_new_work_while_idle", probeNoSpawnWhileIdle},
	{"no_delayed_work_while_idle", probeDelayedWork},
	{"write_semantics", probeWriteSemantics},
	{"stream_writes_bounded", probeStreamWrites},
	{"capability_outside_exec", probeCapabilityOutsideExec},
	{"cancel_reaches_capability", probeCancel},
	{"timeout_reaches_capability", probeTimeout},
	{"output_truncation", probeTruncation},
	{"stalled_consumer", probeStalledConsumer},
}

func probeEnvironment(ctx context.Context, r *runner) Result {
	return r.pyProbe(ctx, "environment", map[string]any{"canary_key": canaryEnv, "canary": r.canary}, `
keys = sorted(os.environ)
leaked = [k for k in keys if k == CFG["canary_key"] or CFG["canary"] in os.environ[k]]
report("host secret absent", not leaked, "keys: %s" % ",".join(keys))
markers = ("PASSWORD", "SECRET", "TOKEN", "DATABASE", "DSN", "PGPASS", "AWS_", "S3_", "OIDC", "API_KEY", "PRIVATE")
risky = [k for k in keys if any(m in k.upper() for m in markers)]
report("no secret-like variables", not risky, ",".join(risky) or "none")
`)
}

func probeFDs(ctx context.Context, r *runner) Result {
	return r.pyProbe(ctx, "fds", nil, `
open_fds = []
for fd in range(0, 4096):
    try:
        os.fstat(fd)
        open_fds.append(fd)
    except OSError:
        pass
report("only stdio and the bridge are inherited", set(open_fds) <= {0, 1, 2, 3}, "open: %s" % open_fds)
`)
}

func probeReadProc(ctx context.Context, r *runner) Result {
	return r.pyProbe(ctx, "proc", nil, `
# These processes exist; procfs hides a process the sandbox may not inspect
# (ENOENT), which is as good as a refusal.
denied("read /proc/1/environ", lambda: read("/proc/1/environ"), (errno.ENOENT,))
denied("read host /proc/<pid>/environ", lambda: read("/proc/%d/environ" % CFG["host_pid"]), (errno.ENOENT,))
denied("read host /proc/<pid>/cmdline", lambda: read("/proc/%d/cmdline" % CFG["host_pid"]), (errno.ENOENT,))
denied("read /proc/self/environ", lambda: read("/proc/self/environ"), (errno.ENOENT,))
denied("list /proc", lambda: os.listdir("/proc"))
`)
}

func probeReadHome(ctx context.Context, r *runner) Result {
	cfg := map[string]any{}
	if home, err := os.UserHomeDir(); err == nil {
		cfg["home"] = home
		if entries, err := os.ReadDir(home); err == nil {
			for _, e := range entries {
				if e.Type().IsRegular() {
					cfg["home_file"] = filepath.Join(home, e.Name())
					break
				}
			}
		}
	}
	if exe, err := os.Executable(); err == nil {
		cfg["app_binary"] = exe
	}
	if cwd, err := os.Getwd(); err == nil {
		cfg["app_dir"] = cwd
	}
	if fi, err := os.Stat("/data"); err == nil && fi.IsDir() {
		cfg["data"] = "/data"
	}
	return r.pyProbe(ctx, "home", cfg, `
if CFG.get("home"):
    denied("list $HOME", lambda: os.listdir(CFG["home"]))
if CFG.get("home_file"):
    denied("read a file in $HOME", lambda: read(CFG["home_file"]))
if CFG.get("app_binary"):
    denied("read the application binary", lambda: read(CFG["app_binary"]))
if CFG.get("app_dir"):
    denied("list the application directory", lambda: os.listdir(CFG["app_dir"]))
if CFG.get("data"):
    denied("list /data", lambda: os.listdir(CFG["data"]))
for p in ("/etc/passwd", "/etc/hostname", "/etc/hosts", "/etc/shadow"):
    if os.path.exists(p):
        denied("read " + p, lambda p=p: read(p))
denied("list /", lambda: os.listdir("/"))
denied("list /tmp", lambda: os.listdir("/tmp"))
`)
}

func probeWrite(ctx context.Context, r *runner) Result {
	cfg := map[string]any{"tmp": filepath.Join(os.TempDir(), "pykernel-conformance-"+randHex())}
	if home, err := os.UserHomeDir(); err == nil {
		cfg["home"] = home
	}
	return r.pyProbe(ctx, "write", cfg, `
wd = CFG["workdir"]
try:
    write(os.path.join(wd, "inside.txt"))
    report("write inside the workdir", True)
except OSError as e:
    report("write inside the workdir", False, e)
denied("write next to the workdir", lambda: write(os.path.join(os.path.dirname(wd), "escape-" + os.path.basename(wd))))
denied("write to " + CFG["tmp"], lambda: write(CFG["tmp"]))
if CFG.get("home"):
    denied("write to $HOME", lambda: write(os.path.join(CFG["home"], ".pykernel-conformance")))
denied("symlink escape", lambda: (os.symlink("/etc/passwd", os.path.join(wd, "link")), read(os.path.join(wd, "link"))))
`)
}

func probeExecWorkdir(ctx context.Context, r *runner) Result {
	return r.pyProbe(ctx, "exec", nil, `
import shutil, subprocess
dst = os.path.join(CFG["workdir"], "python-copy")
try:
    shutil.copyfile(os.path.realpath(sys.executable), dst)
    os.chmod(dst, 0o755)
    denied("execve of a binary written to the workdir", lambda: subprocess.run([dst, "-c", "pass"], check=True))
except OSError as e:
    report("execve of a binary written to the workdir", e.errno in (errno.EACCES, errno.EPERM), e)
`)
}

func (r *runner) dbAddrs() []string {
	addrs := append([]string{"127.0.0.1:5432"}, r.opts.DatabaseAddrs...)
	if h := os.Getenv("DB_HOST"); h != "" {
		port := os.Getenv("DB_PORT")
		if port == "" {
			port = "5432"
		}
		addrs = append(addrs, net.JoinHostPort(h, port))
	}
	var out []string
	seen := map[string]bool{}
	for _, a := range addrs {
		host, port, err := net.SplitHostPort(a)
		if err != nil {
			continue
		}
		if ip := net.ParseIP(host); ip == nil {
			if ips, err := net.LookupIP(host); err == nil && len(ips) > 0 {
				host = ips[0].String()
			}
		}
		a = net.JoinHostPort(host, port)
		if !seen[a] {
			seen[a] = true
			out = append(out, a)
		}
	}
	return out
}

func probeTCP(ctx context.Context, r *runner) Result {
	res := r.pyProbe(ctx, "tcp", map[string]any{
		"external": r.opts.ExternalAddr, "trap": r.tcp.addr(), "databases": r.dbAddrs(),
	}, `
def fam(a):
    return socket.AF_INET6 if ":" in addr(a)[0] else socket.AF_INET
denied("TCP to the internet " + CFG["external"], lambda: connect(fam(CFG["external"]), socket.SOCK_STREAM, addr(CFG["external"])))
denied("TCP to a host listener " + CFG["trap"], lambda: connect(socket.AF_INET, socket.SOCK_STREAM, addr(CFG["trap"])))
for db in CFG["databases"]:
    denied("TCP to the database " + db, lambda db=db: connect(fam(db), socket.SOCK_STREAM, addr(db)))
denied("TCP over IPv6 loopback", lambda: connect(socket.AF_INET6, socket.SOCK_STREAM, ("::1", 5432)))
def bind():
    s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    try:
        s.bind(("0.0.0.0", 0))
        s.listen(1)
    finally:
        s.close()
denied("listen on TCP", bind)
`)
	res.Checks = append(res.Checks, Check{Name: "host listener received no connection", OK: r.tcp.hits.Load() == 0,
		Detail: fmt.Sprintf("%d connections", r.tcp.hits.Load())})
	return res
}

func probeUDP(ctx context.Context, r *runner) Result {
	trap := r.udp.c.LocalAddr().String()
	res := r.pyProbe(ctx, "udp", map[string]any{"external": r.opts.ExternalUDPAddr, "trap": trap}, `
denied("UDP to the internet " + CFG["external"], lambda: connect(socket.AF_INET, socket.SOCK_DGRAM, addr(CFG["external"])))
denied("UDP to a host listener " + CFG["trap"], lambda: connect(socket.AF_INET, socket.SOCK_DGRAM, addr(CFG["trap"])))
`)
	time.Sleep(200 * time.Millisecond)
	res.Checks = append(res.Checks, Check{Name: "host UDP listener received nothing", OK: r.udp.hits.Load() == 0,
		Detail: fmt.Sprintf("%d datagrams", r.udp.hits.Load())})
	return res
}

func probeRaw(ctx context.Context, r *runner) Result {
	return r.pyProbe(ctx, "raw", nil, `
denied("raw ICMP socket", lambda: socket.socket(socket.AF_INET, socket.SOCK_RAW, socket.IPPROTO_ICMP).close())
denied("packet socket", lambda: socket.socket(socket.AF_PACKET, socket.SOCK_RAW, 0).close())
denied("netlink socket", lambda: socket.socket(socket.AF_NETLINK, socket.SOCK_RAW, 0).close())
denied("unix socket creation", lambda: socket.socket(socket.AF_UNIX, socket.SOCK_STREAM).close())
`)
}

func probeUnix(ctx context.Context, r *runner) Result {
	cfg := map[string]any{"path": r.sockPath, "dgram": r.dgramPath}
	if r.abstract != nil {
		cfg["abstract"] = strings.TrimPrefix(r.abstract.addr(), "@")
	}
	res := r.pyProbe(ctx, "unix", cfg, `
if CFG.get("abstract"):
    denied("connect to a host abstract unix socket", lambda: connect(socket.AF_UNIX, socket.SOCK_STREAM, "\0" + CFG["abstract"]))
denied("connect to a host unix socket file", lambda: connect(socket.AF_UNIX, socket.SOCK_STREAM, CFG["path"]))
# A socketpair is creatable; it must not reach any pathname endpoint.
a, b = socket.socketpair(socket.AF_UNIX, socket.SOCK_DGRAM)
denied("sendto a host datagram socket from a socketpair", lambda: a.sendto(b"x", CFG["dgram"]))
denied("sendmsg to a host datagram socket from a socketpair", lambda: a.sendmsg([b"x"], [], 0, CFG["dgram"]))
denied("connect a socketpair to a host datagram socket", lambda: a.connect(CFG["dgram"]))
denied("connect a socketpair to a host stream socket", lambda: b.connect(CFG["path"]))
denied("bind a socketpair in the workdir", lambda: a.bind(os.path.join(CFG["workdir"], "s.sock")))
try:
    a.send(b"ok")
    report("a socketpair still works locally", b.recv(2) == b"ok")
except OSError as e:
    report("a socketpair still works locally", False, e)
a.close(); b.close()
`)
	if r.dgram != nil {
		time.Sleep(200 * time.Millisecond)
		res.Checks = append(res.Checks, Check{Name: "host datagram socket received nothing", OK: r.dgram.hits.Load() == 0,
			Detail: fmt.Sprintf("%d datagrams", r.dgram.hits.Load())})
	}
	for _, t := range []*tcpTrap{r.abstract, r.pathUnix} {
		if t != nil {
			res.Checks = append(res.Checks, Check{Name: "host " + t.addr() + " received no connection", OK: t.hits.Load() == 0,
				Detail: fmt.Sprintf("%d connections", t.hits.Load())})
		}
	}
	return res
}

func probePtrace(ctx context.Context, r *runner) Result {
	return r.pyProbe(ctx, "ptrace", nil, `
hp = CFG["host_pid"]
denied("ptrace(PTRACE_ATTACH, host)", lambda: checked(_libc.ptrace(16, hp, None, None)))
denied("ptrace(PTRACE_TRACEME)", lambda: checked(_libc.ptrace(0, 0, None, None)))
class iovec(ctypes.Structure):
    _fields_ = [("base", ctypes.c_void_p), ("len", ctypes.c_size_t)]
buf = ctypes.create_string_buffer(8)
local = iovec(ctypes.cast(buf, ctypes.c_void_p), 8)
remote = iovec(0x400000, 8)
if SYS:
    denied("process_vm_readv(host)", lambda: raw_syscall(SYS["process_vm_readv"], hp, ctypes.addressof(local), 1, ctypes.addressof(remote), 1, 0))
denied("signal the host process", lambda: os.kill(hp, 0))
denied("signal pid 1", lambda: os.kill(1, 0))
`)
}

func probeSyscalls(ctx context.Context, r *runner) Result {
	return r.pyProbe(ctx, "syscalls", nil, `
CLONE_NEWUSER, CLONE_NEWNET, CLONE_NEWNS = 0x10000000, 0x40000000, 0x00020000
denied("unshare(CLONE_NEWUSER)", lambda: checked(_libc.unshare(CLONE_NEWUSER)))
denied("unshare(CLONE_NEWNET)", lambda: checked(_libc.unshare(CLONE_NEWNET)))
denied("mount", lambda: checked(_libc.mount(b"none", CFG["workdir"].encode(), b"tmpfs", 0, None)))
denied("setsid", lambda: os.setsid())
denied("setpgid", lambda: os.setpgid(0, 0))
if SYS:
    denied("setns", lambda: raw_syscall(SYS["setns"], 0, CLONE_NEWNET))
    denied("bpf", lambda: raw_syscall(SYS["bpf"], 0, 0, 0))
    denied("keyctl", lambda: raw_syscall(SYS["keyctl"], 0, 0))
    denied("add_key", lambda: raw_syscall(SYS["add_key"], 0, 0, 0, 0, 0))
    denied("perf_event_open", lambda: raw_syscall(SYS["perf_event_open"], 0, 0, -1, -1, 0))
    denied("io_uring_setup", lambda: raw_syscall(SYS["io_uring_setup"], 1, 0))
else:
    report("known architecture", False, platform.machine())
`)
}

func probePrivileges(ctx context.Context, r *runner) Result {
	return r.pyProbe(ctx, "privileges", nil, `
PR_GET_NO_NEW_PRIVS = 39
report("no_new_privs is set", _libc.prctl(PR_GET_NO_NEW_PRIVS, 0, 0, 0, 0) == 1)
if SYS:
    header = (ctypes.c_uint32 * 2)(0x20080522, 0)
    data = (ctypes.c_uint32 * 6)()
    raw_syscall(SYS["capget"], ctypes.addressof(header), ctypes.addressof(data))
    report("no capabilities", not any(data), list(data))
report("not running as root", os.getuid() != 0 and os.geteuid() != 0, "uid=%d euid=%d" % (os.getuid(), os.geteuid()))
`)
}

func probeRlimits(ctx context.Context, r *runner) Result {
	return r.pyProbe(ctx, "rlimits", nil, `
import resource
for name in ("RLIMIT_AS", "RLIMIT_CPU", "RLIMIT_FSIZE", "RLIMIT_NPROC", "RLIMIT_NOFILE"):
    soft, hard = resource.getrlimit(getattr(resource, name))
    report(name + " is finite and soft == hard", hard != resource.RLIM_INFINITY and soft == hard, (soft, hard))
    try:
        resource.setrlimit(getattr(resource, name), (hard, hard + 1) if hard != resource.RLIM_INFINITY else (hard, hard))
        report(name + " cannot be raised", False, "raised")
    except (OSError, ValueError) as e:
        report(name + " cannot be raised", True, e)
`)
}

func probeMemory(ctx context.Context, r *runner) Result {
	return r.pyProbe(ctx, "memory", nil, `
import resource
limit = resource.getrlimit(resource.RLIMIT_AS)[0]
report("RLIMIT_AS is set", limit != resource.RLIM_INFINITY, limit)
if limit != resource.RLIM_INFINITY:
    try:
        blob = bytearray(limit)
        report("allocating the whole address space fails", False, "allocated %d bytes" % len(blob))
    except MemoryError:
        report("allocating the whole address space fails", True, "MemoryError")
    chunks = []
    try:
        while True:
            chunks.append(bytearray(64 << 20))
    except MemoryError:
        report("growing until the limit raises MemoryError", True, "%d MiB" % (len(chunks) * 64))
    finally:
        chunks.clear()
`)
}

func probeForkBomb(ctx context.Context, r *runner) Result {
	return r.withKernel(ctx, "forkbomb", nil, func(k pykernel.Kernel, s *session) Result {
		pgid := k.Info().PID
		o, err := r.exec(ctx, k, s, nil, `
import time
root = os.getpid()
forked = 0
while True:
    try:
        pid = os.fork()
    except OSError as e:
        if os.getpid() == root:
            report("fork bomb hits the process limit", e.errno == errno.EAGAIN, "forked %d, then %s" % (forked, e))
            break
        time.sleep(3600)
        os._exit(0)
    if pid == 0:
        continue
    if os.getpid() == root:
        forked += 1
        if forked > 100000:
            report("fork bomb hits the process limit", False, "forked %d processes" % forked)
            break
`, r.opts.ProbeTimeout)
		if err != nil {
			return Result{Error: err.Error()}
		}
		res := Result{Checks: o.checks}
		if len(o.checks) == 0 {
			res.Checks = append(res.Checks, Check{Name: "fork bomb hits the process limit", OK: false,
				Detail: fmt.Sprintf("no report; reason %s", reasonOf(o.done))})
		}
		_ = r.opts.Manager.Release(k)
		_ = r.opts.Manager.Evict(s.key)
		gone := eventually(10*time.Second, func() bool { return groupGone(pgid) })
		res.Checks = append(res.Checks, Check{Name: "every forked process is reaped on dispose", OK: gone,
			Detail: fmt.Sprintf("process group %d", pgid)})
		follow := r.pyProbe(ctx, "after-forkbomb", nil, `report("a new kernel still starts and runs", True)`)
		res.Checks = append(res.Checks, follow.Checks...)
		if follow.Error != "" {
			res.Checks = append(res.Checks, Check{Name: "a new kernel still starts and runs", Detail: follow.Error})
		}
		return res
	})
}

// quotaFill writes 1 MiB chunks into new files (each below RLIMIT_FSIZE)
// until a write fails, and reports how far it got.
const quotaFill = `
import resource
fsize = resource.getrlimit(resource.RLIMIT_FSIZE)[0]
per_file = min(64 << 20, fsize if fsize != resource.RLIM_INFINITY else 64 << 20)
chunk = b"\0" * (1 << 20)
def fill(prefix, unlink=False):
    written, n = 0, 0
    try:
        while written < CFG["max"]:
            path = os.path.join(CFG["workdir"], "%s-%d" % (prefix, n))
            f = open(path, "wb", buffering=0)
            if unlink:
                os.unlink(path)  # the data stays on disk while f is open
                held.append(f)
            size = 0
            while size + len(chunk) <= per_file and written < CFG["max"]:
                got = f.write(chunk)
                size += got
                written += got
                if got < len(chunk):
                    raise OSError(errno.EDQUOT, "short write at the quota")
            if not unlink:
                f.close()
            n += 1
    except OSError as e:
        return written, e.errno
    return written, 0
held = []
`

func (r *runner) quotaLimit() int64 {
	if l, ok := r.opts.Manager.(interface {
		Limits() isolation.ResourceLimits
	}); ok {
		return l.Limits().WorkdirBytes
	}
	return 0
}

// usageOf is the disk the workdir really occupies: allocated blocks of every
// entry (files, directories, symlinks), or the apparent size where larger.
func usageOf(dir string) int64 {
	var total int64
	_ = filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr // a vanished entry is not an error for a measurement
		}
		used := int64(0)
		if st, ok := info.Sys().(*syscall.Stat_t); ok {
			used = st.Blocks * 512
		}
		if info.Mode().IsRegular() && info.Size() > used {
			used = info.Size()
		}
		total += used
		return nil
	})
	return total
}

func (r *runner) quotaChecks(written int64, workdir string) []Check {
	limit := r.quotaLimit()
	if limit == 0 {
		return []Check{{Name: "the Manager reports its quota", Detail: "Limits() unavailable"}}
	}
	return []Check{
		{Name: "the workdir never exceeds the quota", OK: usageOf(workdir) <= limit,
			Detail: fmt.Sprintf("usage %d, quota %d", usageOf(workdir), limit)},
		{Name: "no write beyond the quota was accepted", OK: written <= limit,
			Detail: fmt.Sprintf("written %d, quota %d", written, limit)},
	}
}

func probeQuota(ctx context.Context, r *runner) Result {
	return r.withKernel(ctx, "quota", nil, func(k pykernel.Kernel, s *session) Result {
		o, err := r.exec(ctx, k, s, map[string]any{"max": r.opts.QuotaProbeMaxBytes}, quotaFill+`
written, err = fill("fill")
report("a write past the quota fails with EDQUOT", err == errno.EDQUOT, "written %d, errno %s" % (written, err))
print("WRITTEN %d" % written)
raise OSError(err, "quota")
`, r.opts.ProbeTimeout)
		if err != nil {
			return Result{Error: err.Error()}
		}
		res := Result{Checks: o.checks}
		res.Checks = append(res.Checks, Check{Name: "the exec ends with reason quota_exceeded",
			OK: o.done != nil && o.done.Reason == pykernel.ReasonQuotaExceeded, Detail: reasonOf(o.done)})
		res.Checks = append(res.Checks, r.quotaChecks(writtenOf(o.stdout), s.workdir)...)
		return res
	})
}

func probeQuotaUnlinked(ctx context.Context, r *runner) Result {
	return r.withKernel(ctx, "quota-unlinked", nil, func(k pykernel.Kernel, s *session) Result {
		o, err := r.exec(ctx, k, s, map[string]any{"max": r.opts.QuotaProbeMaxBytes}, quotaFill+`
written, err = fill("hidden", unlink=True)
report("writes to unlinked open files count against the quota", err == errno.EDQUOT, "written %d, errno %s" % (written, err))
print("WRITTEN %d" % written)
`, r.opts.ProbeTimeout)
		if err != nil {
			return Result{Error: err.Error()}
		}
		res := Result{Checks: o.checks}
		res.Checks = append(res.Checks, r.quotaChecks(writtenOf(o.stdout), s.workdir)...)
		return res
	})
}

func writtenOf(stdout string) int64 {
	for _, line := range strings.Split(stdout, "\n") {
		if v, ok := strings.CutPrefix(line, "WRITTEN "); ok {
			n, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
			return n
		}
	}
	return -1
}

func probeQuotaAfterExec(ctx context.Context, r *runner) Result {
	return r.withKernel(ctx, "quota-after", nil, func(k pykernel.Kernel, s *session) Result {
		o, err := r.exec(ctx, k, s, map[string]any{"max": r.opts.QuotaProbeMaxBytes}, quotaFill+`
import threading
def bg():
    global bg_state
    bg_state = fill("bg")
threading.Thread(target=bg, daemon=True).start()
report("exec returned while a thread keeps writing", True)
`, r.opts.ProbeTimeout)
		if err != nil {
			return Result{Error: err.Error()}
		}
		res := Result{Checks: o.checks}
		// Either the writer hits the quota, or the idle guard kills a kernel
		// that keeps working after its exec ended. Both hold the quota.
		var outcome string
		eventually(r.opts.ProbeTimeout, func() bool {
			ch, err := k.Exec(ctx, pykernel.ExecRequest{Code: "print('STATE', globals().get('bg_state'))", Timeout: 5 * time.Second})
			if err != nil {
				outcome = "killed"
				return errors.Is(err, pykernel.ErrKernelDisposed)
			}
			out := collect(ch).stdout
			if !strings.Contains(out, "STATE None") {
				outcome = strings.TrimSpace(out)
				return true
			}
			time.Sleep(200 * time.Millisecond)
			return false
		})
		ok := outcome == "killed" || strings.HasSuffix(outcome, fmt.Sprintf(", %d)", syscall.EDQUOT))
		res.Checks = append(res.Checks, Check{Name: "a writer between execs hits the quota or its kernel is killed", OK: ok, Detail: outcome})
		res.Checks = append(res.Checks, Check{Name: "the workdir never exceeds the quota", OK: r.quotaLimit() > 0 && usageOf(s.workdir) <= r.quotaLimit(),
			Detail: fmt.Sprintf("usage %d, quota %d", usageOf(s.workdir), r.quotaLimit())})
		return res
	})
}

func probeForgedDone(ctx context.Context, r *runner) Result {
	return r.withKernel(ctx, "forged-done", nil, func(k pykernel.Kernel, s *session) Result {
		const deadline = 3 * time.Second
		start := time.Now()
		ch, err := k.Exec(ctx, pykernel.ExecRequest{Code: `
import __main__
# The interpreter owns the bridge: claim the exec is done, then keep working.
__main__._notify("exec.done", {"exec_id": __main__._current_exec})
while True:
    pass
`, Timeout: deadline})
		if err != nil {
			return Result{Error: err.Error()}
		}
		o := collect(ch)
		took := time.Since(start)
		res := Result{Checks: []Check{
			{Name: "a done claimed by the worker does not end a running exec", OK: took >= deadline,
				Detail: fmt.Sprintf("stream ended after %s", took.Round(time.Millisecond))},
			{Name: "the exec ends at its deadline with reason timeout", OK: o.done != nil && o.done.Reason == pykernel.ReasonTimeout,
				Detail: reasonOf(o.done)},
		}}
		// Interrupted or killed: either way the loop must not run any more.
		pid := k.Info().PID
		before := groupCPU(pid)
		time.Sleep(time.Second)
		_, alive := r.opts.Manager.Get(s.key)
		burnt := groupCPU(pid) - before
		res.Checks = append(res.Checks, Check{Name: "the runaway code no longer runs", OK: !alive || burnt < 200*time.Millisecond,
			Detail: fmt.Sprintf("alive=%t, CPU in the next second %s", alive, burnt)})
		return res
	})
}

func blockingCapability(seen chan<- error) pykernel.Capability {
	return pykernel.CapabilityFunc("block", pykernel.AccessRead, pykernel.CapabilitySignature{},
		func(ctx context.Context, _ pykernel.CallArgs) (any, error) {
			select {
			case <-ctx.Done():
				seen <- ctx.Err()
				return nil, ctx.Err()
			case <-time.After(2 * time.Minute):
				seen <- nil
				return "never cancelled", nil
			}
		})
}

func probeCancel(ctx context.Context, r *runner) Result {
	seen := make(chan error, 1)
	return r.withKernel(ctx, "cancel", []pykernel.Capability{blockingCapability(seen)}, func(k pykernel.Kernel, s *session) Result {
		execCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		ch, err := k.Exec(execCtx, pykernel.ExecRequest{Code: "block()"})
		if err != nil {
			return Result{Error: err.Error()}
		}
		time.AfterFunc(300*time.Millisecond, cancel)
		var res Result
		select {
		case err := <-seen:
			res.Checks = append(res.Checks, Check{Name: "cancelling the exec cancels the capability call", OK: err != nil, Detail: fmt.Sprint(err)})
		case <-time.After(20 * time.Second):
			res.Checks = append(res.Checks, Check{Name: "cancelling the exec cancels the capability call", Detail: "capability still running after 20s"})
		}
		o := collect(ch)
		res.Checks = append(res.Checks, Check{Name: "the exec ends with reason cancelled",
			OK: o.done != nil && o.done.Reason == pykernel.ReasonCancelled, Detail: reasonOf(o.done)})
		res.Checks = append(res.Checks, r.slotFree(ctx, k, s))
		return res
	})
}

func probeTimeout(ctx context.Context, r *runner) Result {
	seen := make(chan error, 1)
	return r.withKernel(ctx, "timeout", []pykernel.Capability{blockingCapability(seen)}, func(k pykernel.Kernel, s *session) Result {
		ch, err := k.Exec(ctx, pykernel.ExecRequest{Code: "block()", Timeout: 500 * time.Millisecond})
		if err != nil {
			return Result{Error: err.Error()}
		}
		var res Result
		select {
		case err := <-seen:
			res.Checks = append(res.Checks, Check{Name: "the timeout cancels the capability call", OK: err != nil, Detail: fmt.Sprint(err)})
		case <-time.After(20 * time.Second):
			res.Checks = append(res.Checks, Check{Name: "the timeout cancels the capability call", Detail: "capability still running after 20s"})
		}
		o := collect(ch)
		res.Checks = append(res.Checks, Check{Name: "the exec ends with reason timeout",
			OK: o.done != nil && o.done.Reason == pykernel.ReasonTimeout, Detail: reasonOf(o.done)})
		res.Checks = append(res.Checks, r.slotFree(ctx, k, s))
		return res
	})
}

// slotFree checks that the kernel serves the next exec after a stopped one.
func (r *runner) slotFree(ctx context.Context, k pykernel.Kernel, s *session) Check {
	ok := eventually(10*time.Second, func() bool { return !k.Info().InFlight })
	if !ok {
		return Check{Name: "the slot is freed for the next exec", Detail: "still in flight"}
	}
	o, err := r.exec(ctx, k, s, nil, `report("next exec", True)`, 20*time.Second)
	if err != nil {
		return Check{Name: "the slot is freed for the next exec", Detail: err.Error()}
	}
	return Check{Name: "the slot is freed for the next exec", OK: len(o.checks) == 1 && o.checks[0].OK, Detail: reasonOf(o.done)}
}

func probeTruncation(ctx context.Context, r *runner) Result {
	return r.withKernel(ctx, "truncation", nil, func(k pykernel.Kernel, _ *session) Result {
		ch, err := k.Exec(ctx, pykernel.ExecRequest{Code: "print('x' * 10000)", OutputCap: 1024})
		if err != nil {
			return Result{Error: err.Error()}
		}
		o := collect(ch)
		return Result{Checks: []Check{
			{Name: "output beyond the cap is cut", OK: len(o.stdout) <= 1024, Detail: fmt.Sprintf("%d bytes", len(o.stdout))},
			{Name: "EventDone reports the truncation", OK: o.done != nil && o.done.StdoutTruncated && o.done.Truncated(), Detail: reasonOf(o.done)},
		}}
	})
}

func probeStalledConsumer(ctx context.Context, r *runner) Result {
	return r.withKernel(ctx, "stalled", nil, func(k pykernel.Kernel, s *session) Result {
		ch, err := k.Exec(ctx, pykernel.ExecRequest{Code: "for i in range(5000):\n    print(i, flush=True)", OutputCap: 1 << 20})
		if err != nil {
			return Result{Error: err.Error()}
		}
		var res Result
		finished := eventually(30*time.Second, func() bool { return !k.Info().InFlight })
		res.Checks = append(res.Checks, Check{Name: "an exec whose consumer stopped reading still finishes", OK: finished})
		if finished {
			res.Checks = append(res.Checks, r.slotFree(ctx, k, s))
		}
		var done *pykernel.DonePayload
		for ev := range ch {
			if ev.Kind == pykernel.EventDone {
				done = ev.Done
			}
		}
		res.Checks = append(res.Checks, Check{Name: "the stalled stream still ends with EventDone reporting dropped events",
			OK: done != nil && done.EventsDropped > 0 && done.Truncated(), Detail: reasonOf(done)})
		return res
	})
}

func reasonOf(d *pykernel.DonePayload) string {
	if d == nil {
		return "no EventDone"
	}
	return fmt.Sprintf("reason=%s stdout_truncated=%t dropped=%d", d.Reason, d.StdoutTruncated, d.EventsDropped)
}

func eventually(d time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(d)
	for {
		if cond() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func probeSibling(ctx context.Context, r *runner) Result {
	sib := filepath.Join(r.root, "sibling-"+randHex())
	if err := os.MkdirAll(sib, 0o700); err != nil {
		return Result{Error: err.Error()}
	}
	secret := filepath.Join(sib, "secret.txt")
	if err := os.WriteFile(secret, []byte("other tenant"), 0o600); err != nil {
		return Result{Error: err.Error()}
	}
	return r.pyProbe(ctx, "sibling", map[string]any{"dir": sib, "file": secret}, `
denied("list another session's workdir", lambda: os.listdir(CFG["dir"]))
denied("read another session's file", lambda: read(CFG["file"]))
denied("write into another session's workdir", lambda: write(os.path.join(CFG["dir"], "planted")))
`)
}

func probeCapabilityOutsideExec(ctx context.Context, r *runner) Result {
	called := make(chan struct{}, 4)
	probeCap := pykernel.CapabilityFunc("touch", pykernel.AccessRead, pykernel.CapabilitySignature{},
		func(context.Context, pykernel.CallArgs) (any, error) {
			called <- struct{}{}
			return "ran", nil
		})
	return r.withKernel(ctx, "outside-exec", []pykernel.Capability{probeCap}, func(k pykernel.Kernel, s *session) Result {
		o, err := r.exec(ctx, k, s, nil, `
import threading, time
def late():
    time.sleep(0.5)
    try:
        touch()
        outcome = "ran"
    except Exception as e:
        outcome = type(e).__name__
    with open(os.path.join(CFG["workdir"], "late.txt"), "w") as f:
        f.write(outcome)
threading.Thread(target=late, daemon=True).start()
report("exec started a thread that calls a capability later", True)
`, r.opts.ProbeTimeout)
		if err != nil {
			return Result{Error: err.Error()}
		}
		res := Result{Checks: o.checks}
		// The kernel is frozen once the exec ends: the thread cannot run, let
		// alone reach a capability, until a later exec bounds it.
		time.Sleep(2 * time.Second)
		_, err = os.Stat(filepath.Join(s.workdir, "late.txt"))
		res.Checks = append(res.Checks, Check{Name: "no capability call happens after the exec ended",
			OK: len(called) == 0 && os.IsNotExist(err), Detail: fmt.Sprintf("handler ran %d times", len(called))})
		return res
	})
}

// quotaAttacks are known ways around a naive quota; each prints how many
// bytes it managed to allocate, which must stay within the quota.
var quotaAttacks = []struct{ name, code string }{
	{"dup2 a workdir file onto stdout", `
total = 0
try:
    for i in range(100000):
        fd = os.open("d%d" % i, os.O_WRONLY | os.O_CREAT, 0o600)
        os.dup2(fd, 1)
        os.close(fd)
        for _ in range(per_file // len(chunk)):
            total += os.write(1, chunk)
except OSError:
    pass
`},
	{"pwrite at offset 0 on an O_APPEND file", `
total = 0
try:
    for i in range(100000):
        fd = os.open("a%d" % i, os.O_WRONLY | os.O_CREAT | os.O_APPEND, 0o600)
        for _ in range(per_file // len(chunk)):
            total += os.pwrite(fd, chunk, 0)
        os.close(fd)
except OSError:
    pass
`},
	{"copy_file_range with a huge length", `
total = 0
try:
    src = os.open("src", os.O_RDWR | os.O_CREAT, 0o600)
    total += os.write(src, chunk)
    for i in range(100000):
        dst = os.open("c%d" % i, os.O_WRONLY | os.O_CREAT, 0o600)
        os.lseek(src, 0, 0)
        total += os.copy_file_range(src, dst, 2**62)
except OSError:
    pass
`},
	{"race the iovec length from another thread", `
import threading
class iovec(ctypes.Structure):
    _fields_ = [("base", ctypes.c_void_p), ("len", ctypes.c_size_t)]
buf = ctypes.create_string_buffer(len(chunk))
iov = iovec(ctypes.cast(buf, ctypes.c_void_p), 0)
stop = False
def flip():
    while not stop:
        iov.len = 0
        iov.len = len(chunk)
threading.Thread(target=flip, daemon=True).start()
_libc.writev.restype = ctypes.c_ssize_t
total, failed = 0, False
for i in range(64):
    fd = os.open("r%d" % i, os.O_WRONLY | os.O_CREAT, 0o600)
    for _ in range(per_file // len(chunk)):
        n = _libc.writev(fd, ctypes.byref(iov), 1)
        if n < 0:
            failed = True
            break
        total += n
    os.close(fd)
    if failed:
        break
stop = True
`},
	{"pwritev2 RWF_APPEND from a position far past the end", `
total = 0
fd = os.open("sparse", os.O_RDWR | os.O_CREAT, 0o600)
os.lseek(fd, 20 << 30, 0)
try:
    os.pwritev(fd, [b"x"], -1, 0x10)
except OSError:
    pass
total = os.fstat(fd).st_size
`},
	{"race the file position from another thread", `
import threading
fd = os.open("race", os.O_RDWR | os.O_CREAT, 0o600)
stop = False
def flip():
    while not stop:
        os.lseek(fd, 0, 0)
        os.lseek(fd, 10 << 30, 0)
threading.Thread(target=flip, daemon=True).start()
total = 0
for i in range(2000):
    try:
        os.write(fd, b"y")
    except OSError:
        pass
    total = max(total, os.fstat(fd).st_size)
stop = True
`},
	{"fill the holes of a sparse file through a shared mapping", `
import mmap
total = 0
try:
    fd = os.open("holes", os.O_RDWR | os.O_CREAT, 0o600)
    os.lseek(fd, 20 << 30, 0)
    try:
        os.pwritev(fd, [b"x"], -1, 0x10)
    except OSError:
        pass
    size = os.fstat(fd).st_size
    if size > 0:
        m = mmap.mmap(fd, size)
        for off in range(0, size, 4096):
            m[off] = 1
        m.flush()
    total = size
except (OSError, ValueError):
    pass
`},
	{"flood the workdir with directories and long symlinks", `
import time
total, deadline, refused = 0, time.monotonic() + CFG["flood_seconds"], None
# Fill bytes to just below the quota first, so the entry flood reaches the
# boundary within its deadline at any machine speed.
left, part = CFG["quota"] - (4 << 20), 0
while left > 0:
    with open("filler-%d" % part, "wb", buffering=0) as f:
        size = 0
        while left > 0 and size < per_file:  # per_file respects RLIMIT_FSIZE
            n = f.write(b"\0" * min(left, 1 << 20, per_file - size))
            if not n:
                break
            size += n
            left -= n
    part += 1
try:
    i = 0
    while time.monotonic() < deadline:
        os.symlink("x" * 4000, "s%d" % i)  # a block each
        if i % 16 == 0:
            os.mkdir("d%d" % i)
        i += 1
except OSError as e:
    refused = e.errno
report("the entry flood is refused by the quota", refused == errno.EDQUOT,
       "%d entries, stopped by %s" % (i, errno.errorcode.get(refused, refused)))
`},
	{"mmap, close and unlink a grown file", `
_libc.mmap.restype = ctypes.c_void_p
_libc.mmap.argtypes = [ctypes.c_void_p, ctypes.c_size_t, ctypes.c_int, ctypes.c_int, ctypes.c_int, ctypes.c_long]
total, maps = 0, []
try:
    for i in range(10000):
        path = "m%d" % i
        fd = os.open(path, os.O_RDWR | os.O_CREAT, 0o600)
        os.ftruncate(fd, per_file)
        p = _libc.mmap(None, per_file, 3, 1, fd, 0)  # PROT_READ|WRITE, MAP_SHARED
        if p in (None, ctypes.c_void_p(-1).value):
            break
        ctypes.memset(p, 1, per_file)
        os.close(fd)
        os.unlink(path)
        maps.append(p)
        total += per_file
except OSError:
    pass
`},
}

func probeQuotaBypasses(ctx context.Context, r *runner) Result {
	var res Result
	limit := r.quotaLimit()
	for _, attack := range quotaAttacks {
		sub := r.withKernel(ctx, "quota-bypass", nil, func(k pykernel.Kernel, s *session) Result {
			flood := r.opts.ProbeTimeout / 2
			o, err := r.exec(ctx, k, s, map[string]any{"max": r.opts.QuotaProbeMaxBytes, "flood_seconds": flood.Seconds(), "quota": limit},
				quotaFill+attack.code+`
print("WRITTEN %d" % total)
`, r.opts.ProbeTimeout)
			if err != nil {
				return Result{Error: err.Error()}
			}
			got := writtenOf(o.stdout)
			usage := usageOf(s.workdir)
			detail := fmt.Sprintf("allocated %d, workdir %d, quota %d; %s", got, usage, limit, reasonOf(o.done))
			if o.err != nil {
				detail += "; raised " + o.err.Error()
			}
			checks := append(o.checks, Check{
				Name:   attack.name + " stays within the quota",
				OK:     limit > 0 && usage <= limit && (got < 0 || got <= limit),
				Detail: detail,
			})
			if got < 0 {
				// The attack did not finish: that is a failure of the probe,
				// reported apart from any limit breach.
				checks = append(checks, Check{Name: attack.name + " finished within the probe deadline", Detail: detail})
			}
			return Result{Checks: checks}
		})
		if sub.Error != "" {
			sub.Checks = append(sub.Checks, Check{Name: attack.name, Detail: sub.Error})
		}
		res.Checks = append(res.Checks, sub.Checks...)
	}
	return res
}

func probeRefusedAllocators(ctx context.Context, r *runner) Result {
	return r.pyProbe(ctx, "allocators", nil, `
import fcntl
fd = os.open(os.path.join(CFG["workdir"], "f"), os.O_RDWR | os.O_CREAT, 0o600)
src = os.open(os.path.join(CFG["workdir"], "g"), os.O_RDWR | os.O_CREAT, 0o600)
def expect(check, fn, errs):
    try:
        fn()
    except OSError as e:
        report(check, e.errno in errs, e)
        return
    report(check, False, "allowed")
_libc.fallocate.argtypes = [ctypes.c_int, ctypes.c_int, ctypes.c_long, ctypes.c_long]
expect("fallocate is refused", lambda: checked(_libc.fallocate(fd, 0, 0, 1 << 20)), (errno.EOPNOTSUPP,))
expect("truncate by path is refused", lambda: os.truncate(os.path.join(CFG["workdir"], "f"), 1 << 20), (errno.EPERM,))
expect("setxattr is refused", lambda: os.setxattr(os.path.join(CFG["workdir"], "f"), "user.x", b"x" * 1000), (errno.EOPNOTSUPP,))
expect("FICLONE is refused", lambda: fcntl.ioctl(fd, 0x40049409, src), (errno.ENOTTY,))
expect("sendfile is refused", lambda: os.sendfile(fd, src, 0, 1), (errno.EINVAL,))
report("isatty still works", os.isatty(fd) is False)
`)
}

func probeIdleForkEvasion(ctx context.Context, r *runner) Result {
	return r.withKernel(ctx, "idle-fork", nil, func(k pykernel.Kernel, s *session) Result {
		o, err := r.exec(ctx, k, s, nil, `
import threading, time
def burn():
    while True:
        try:
            pid = os.fork()
        except OSError:
            time.sleep(0.05)
            continue
        if pid == 0:
            end = time.time() + 0.7
            while time.time() < end:
                pass
            os._exit(0)
        os.waitpid(pid, 0)
threading.Thread(target=burn, daemon=True).start()
report("exec returned while children keep burning CPU", True)
`, r.opts.ProbeTimeout)
		if err != nil {
			return Result{Error: err.Error()}
		}
		res := Result{Checks: o.checks}
		pid := k.Info().PID
		start := groupCPU(pid)
		time.Sleep(8 * time.Second)
		_, alive := r.opts.Manager.Get(s.key)
		burnt := groupCPU(pid) - start
		res.Checks = append(res.Checks, Check{Name: "background children cannot keep burning CPU while idle",
			OK: !alive || burnt < 2*time.Second, Detail: fmt.Sprintf("alive=%t, CPU used while idle %s", alive, burnt)})
		return res
	})
}

// groupCPU sums user+system CPU, children included, of process group pgid.
func groupCPU(pgid int) time.Duration {
	entries, _ := os.ReadDir("/proc")
	var ticks uint64
	for _, e := range entries {
		raw, err := os.ReadFile("/proc/" + e.Name() + "/stat")
		if err != nil {
			continue
		}
		s := string(raw)
		i := strings.LastIndexByte(s, ')')
		if i < 0 {
			continue
		}
		f := strings.Fields(s[i+1:])
		if len(f) < 15 || f[2] != strconv.Itoa(pgid) {
			continue
		}
		for _, idx := range []int{11, 12, 13, 14} {
			v, _ := strconv.ParseUint(f[idx], 10, 64)
			ticks += v
		}
	}
	return time.Duration(ticks) * time.Second / 100
}

func probeNoSpawnWhileIdle(ctx context.Context, r *runner) Result {
	return r.withKernel(ctx, "idle-spawn", nil, func(k pykernel.Kernel, s *session) Result {
		o, err := r.exec(ctx, k, s, nil, `
import threading, time, signal
_libc.signal(signal.SIGCHLD, 1)  # SIG_IGN from any thread: children would be auto-reaped
def later():
    global spawn_result
    time.sleep(1.5)
    try:
        pid = os.fork()
        if pid == 0:
            os._exit(0)
        spawn_result = "forked"
    except OSError as e:
        spawn_result = errno.errorcode.get(e.errno, str(e.errno))
    try:
        threading.Thread(target=lambda: None).start()
        spawn_result += ",thread"
    except RuntimeError:
        spawn_result += ",nothread"
threading.Thread(target=later, daemon=True).start()
report("exec returned with a timer to fork later", True)
`, r.opts.ProbeTimeout)
		if err != nil {
			return Result{Error: err.Error()}
		}
		res := Result{Checks: o.checks}
		pid := k.Info().PID
		before := groupSize(pid)
		time.Sleep(3 * time.Second)
		after := groupSize(pid)
		res.Checks = append(res.Checks, Check{Name: "no process or thread starts while no exec runs",
			OK: after <= before, Detail: fmt.Sprintf("tasks %d before, %d after the idle period", before, after)})
		return res
	})
}

func probeWriteSemantics(ctx context.Context, r *runner) Result {
	return r.pyProbe(ctx, "semantics", nil, `
import signal
r_, w_ = os.pipe()
pid = os.fork()
if pid == 0:
    os.close(w_)
    got = 0
    while got < 200000:
        d = os.read(r_, 65536)
        if not d:
            break
        got += len(d)
    os._exit(0)
os.close(r_)
report("a blocking pipe write is written in full", os.write(w_, b"p" * 200000) == 200000)
os.close(w_)
os.waitpid(pid, 0)
fd = os.open(os.path.join(CFG["workdir"], "big"), os.O_WRONLY | os.O_CREAT, 0o600)
report("a 3 MiB file write is written in full", os.write(fd, b"f" * (3 << 20)) == 3 << 20)
afd = os.open(os.path.join(CFG["workdir"], "app"), os.O_WRONLY | os.O_CREAT | os.O_APPEND, 0o600)
os.write(afd, b"ab")
before = os.lseek(afd, 0, 1)
os.pwrite(afd, b"cd", 0)
report("pwrite on an O_APPEND file appends and keeps the position",
       os.lseek(afd, 0, 1) == before and os.fstat(afd).st_size == 4)
r2, w2 = os.pipe()
os.close(r2)
pid = os.fork()
if pid == 0:
    signal.signal(signal.SIGPIPE, signal.SIG_DFL)
    try:
        os.write(w2, b"x")
    except OSError:
        os._exit(7)
    os._exit(0)
_, status = os.waitpid(pid, 0)
report("a write to a closed pipe raises SIGPIPE", os.WIFSIGNALED(status) and os.WTERMSIG(status) == signal.SIGPIPE, status)
a, b = socket.socketpair()
a.sendall(b"s" * 300000) if False else None
report("send on a socketpair works", a.send(b"hi") == 2 and b.recv(2) == b"hi")
`)
}

func probeStreamWrites(ctx context.Context, r *runner) Result {
	before := runtime.NumGoroutine()
	res := r.withKernel(ctx, "streams", nil, func(k pykernel.Kernel, s *session) Result {
		o, err := r.exec(ctx, k, s, nil, `
import signal, time, threading
r_, w_ = os.pipe()
os.set_blocking(w_, False)
try:
    while True:
        os.write(w_, b"z" * 4096)
except BlockingIOError:
    pass
os.set_blocking(w_, True)
def writer():
    try:
        os.write(w_, b"q")
    except BaseException:
        pass
t = threading.Thread(target=writer, daemon=True)
t.start()
time.sleep(0.2)
for _ in range(300):
    try:
        signal.pthread_kill(t.ident, signal.SIGINT)
    except BaseException:
        pass
    time.sleep(0.002)
os.set_blocking(r_, False)
got = 0
for _ in range(20):
    try:
        while True:
            got += os.read(r_, 1 << 20).count(b"q")
    except BlockingIOError:
        pass
    time.sleep(0.05)
report("an interrupted blocked write happens exactly once", got == 1, "delivered %d" % got)
fifo = os.path.join(CFG["workdir"], "fifo")
os.mkfifo(fifo)
ffd = os.open(fifo, os.O_RDWR)
os.set_blocking(ffd, False)
try:
    while True:
        os.write(ffd, b"f" * 4096)
except BlockingIOError:
    pass
os.set_blocking(ffd, True)
for _ in range(20):  # more than the host's stream slots
    try:
        threading.Thread(target=lambda: os.write(ffd, b"x"), daemon=True).start()
    except RuntimeError:
        break
time.sleep(0.3)
report("writers blocked on a full FIFO", True)
`, r.opts.ProbeTimeout)
		if err != nil {
			return Result{Error: err.Error()}
		}
		res := Result{Checks: o.checks}
		if o.err != nil {
			res.Error = "probe raised " + o.err.Error()
		}
		return res
	})
	settled := eventually(10*time.Second, func() bool { return runtime.NumGoroutine() <= before+4 })
	res.Checks = append(res.Checks, Check{Name: "blocked stream writes do not hold host goroutines after dispose", OK: settled,
		Detail: fmt.Sprintf("goroutines before %d, after %d", before, runtime.NumGoroutine())})
	return res
}

func probeHostIPC(ctx context.Context, r *runner) Result {
	// A segment of our own, created exclusively: an existing segment with the
	// same key is never adopted, and only what this call created is removed.
	cfg := map[string]any{"key": 0}
	for attempt := 0; attempt < 16; attempt++ {
		b := make([]byte, 4)
		_, _ = rand.Read(b)
		key := int(binary.LittleEndian.Uint32(b)&0x3fffffff) | 1
		id, err := unix.SysvShmGet(key, 4096, unix.IPC_CREAT|unix.IPC_EXCL|0o666)
		if errors.Is(err, unix.EEXIST) {
			continue
		}
		if err == nil {
			cfg["key"] = key
			defer func(id int) { _, _ = unix.SysvShmCtl(id, unix.IPC_RMID, nil) }(id)
		}
		break
	}
	return r.pyProbe(ctx, "ipc", cfg, `
IPC_CREAT, IPC_PRIVATE = 0o1000, 0
if CFG["key"]:
    denied("attach to a host SysV shared memory segment", lambda: raw_syscall(SYS["shmget"], CFG["key"], 0, 0o666))
denied("create SysV shared memory", lambda: raw_syscall(SYS["shmget"], IPC_PRIVATE, 4096, IPC_CREAT | 0o600))
denied("create a SysV message queue", lambda: raw_syscall(SYS["msgget"], IPC_PRIVATE, IPC_CREAT | 0o600))
denied("create SysV semaphores", lambda: raw_syscall(SYS["semget"], IPC_PRIVATE, 1, IPC_CREAT | 0o600))
name = ctypes.create_string_buffer(b"pykernel-conformance")
denied("open a POSIX message queue", lambda: raw_syscall(SYS["mq_open"], ctypes.addressof(name), os.O_RDWR | os.O_CREAT, 0o600, 0))
`)
}

// groupSize counts the tasks (threads of every process) of process group pgid.
func groupSize(pgid int) int {
	entries, _ := os.ReadDir("/proc")
	n := 0
	for _, e := range entries {
		raw, err := os.ReadFile("/proc/" + e.Name() + "/stat")
		if err != nil {
			continue
		}
		s := string(raw)
		i := strings.LastIndexByte(s, ')')
		if i < 0 {
			continue
		}
		f := strings.Fields(s[i+1:])
		if len(f) < 3 || f[2] != strconv.Itoa(pgid) {
			continue
		}
		tasks, _ := os.ReadDir("/proc/" + e.Name() + "/task")
		n += len(tasks)
	}
	return n
}

func probeDelayedWork(ctx context.Context, r *runner) Result {
	return r.withKernel(ctx, "delayed", nil, func(k pykernel.Kernel, s *session) Result {
		o, err := r.exec(ctx, k, s, nil, `
import threading, time
def later():
    time.sleep(0.5)
    with open(os.path.join(CFG["workdir"], "late-work"), "w") as f:
        f.write("ran")
threading.Thread(target=later, daemon=True).start()
report("exec returned with work scheduled for later", True)
`, r.opts.ProbeTimeout)
		if err != nil {
			return Result{Error: err.Error()}
		}
		res := Result{Checks: o.checks}
		pid := k.Info().PID
		cpu := groupCPU(pid)
		time.Sleep(2 * time.Second)
		_, statErr := os.Stat(filepath.Join(s.workdir, "late-work"))
		res.Checks = append(res.Checks, Check{Name: "work left sleeping does not resume after the exec ended",
			OK:     os.IsNotExist(statErr) && groupCPU(pid)-cpu < 50*time.Millisecond,
			Detail: fmt.Sprintf("file written: %t, CPU while idle %s", statErr == nil, groupCPU(pid)-cpu)})
		return res
	})
}

func probeHostProcessControls(ctx context.Context, r *runner) Result {
	return r.pyProbe(ctx, "proc-controls", nil, `
import resource
hp = CFG["host_pid"]
denied("lower a host process's limits with prlimit", lambda: resource.prlimit(hp, resource.RLIMIT_NOFILE, (64, 64)))
denied("read a host process's limits with prlimit", lambda: resource.prlimit(hp, resource.RLIMIT_NOFILE))
denied("renice a host process", lambda: os.setpriority(os.PRIO_PROCESS, hp, 10))
denied("renice every process of the uid", lambda: os.setpriority(os.PRIO_USER, 0, 10))
denied("move a host process to another CPU", lambda: os.sched_setaffinity(hp, {0}))
try:
    soft, hard = resource.prlimit(0, resource.RLIMIT_NOFILE)
    resource.setrlimit(resource.RLIMIT_NOFILE, (soft, hard))
    os.setpriority(os.PRIO_PROCESS, 0, os.getpriority(os.PRIO_PROCESS, 0))
    report("the kernel still adjusts itself", True)
except OSError as e:
    report("the kernel still adjusts itself", False, e)
`)
}
