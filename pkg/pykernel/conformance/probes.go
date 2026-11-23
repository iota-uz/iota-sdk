//go:build unix

package conformance

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/pykernel"
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
    "x86_64": {"bpf": 321, "keyctl": 250, "perf_event_open": 298, "io_uring_setup": 425, "setns": 308,
               "process_vm_readv": 310, "mount": 165, "capget": 125, "add_key": 248},
    "aarch64": {"bpf": 280, "keyctl": 219, "perf_event_open": 241, "io_uring_setup": 425, "setns": 268,
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
    denied("execute a binary written to the workdir", lambda: subprocess.run([dst, "-c", "pass"], check=True))
except OSError as e:
    report("execute a binary written to the workdir", e.errno in (errno.EACCES, errno.EPERM), e)
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
	cfg := map[string]any{"path": r.sockPath}
	if r.abstract != nil {
		cfg["abstract"] = strings.TrimPrefix(r.abstract.addr(), "@")
	}
	res := r.pyProbe(ctx, "unix", cfg, `
if CFG.get("abstract"):
    denied("connect to a host abstract unix socket", lambda: connect(socket.AF_UNIX, socket.SOCK_STREAM, "\0" + CFG["abstract"]))
denied("connect to a host unix socket file", lambda: connect(socket.AF_UNIX, socket.SOCK_STREAM, CFG["path"]))
`)
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

func probeQuota(ctx context.Context, r *runner) Result {
	return r.withKernel(ctx, "quota", nil, func(k pykernel.Kernel, s *session) Result {
		o, err := r.exec(ctx, k, s, map[string]any{"max": r.opts.QuotaProbeMaxBytes}, `
import resource
fsize = resource.getrlimit(resource.RLIMIT_FSIZE)[0]
per_file = min(64 << 20, fsize if fsize != resource.RLIM_INFINITY else 64 << 20)
chunk = b"\0" * (1 << 20)
written, n = 0, 0
while written < CFG["max"]:
    with open(os.path.join(CFG["workdir"], "fill-%d" % n), "wb") as f:
        size = 0
        while size + len(chunk) <= per_file and written < CFG["max"]:
            f.write(chunk)
            size += len(chunk)
            written += len(chunk)
    n += 1
report("writing past the quota is stopped", False, "wrote %d bytes" % written)
`, r.opts.ProbeTimeout)
		if err != nil {
			return Result{Error: err.Error()}
		}
		res := Result{Checks: o.checks}
		res.Checks = append(res.Checks, Check{Name: "the exec ends with reason quota_exceeded",
			OK: o.done != nil && o.done.Reason == pykernel.ReasonQuotaExceeded, Detail: reasonOf(o.done)})
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
