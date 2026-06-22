"""pykernel bootstrap shim.

Runs inside the sandboxed subprocess. Connects to the host control socket,
exposes host capabilities as callable Python functions, executes submitted code
in a persistent namespace, and streams stdout/result/metric/log/done back over
a length-prefixed JSON-RPC channel.

All data access goes through capability calls over the socket: this process has
no database credentials and (in production) no network egress, so importing a
DB driver here is inert. The host enforces the plan/apply boundary and refuses a
write capability in plan mode by returning a JSON-RPC error whose data.type this
shim raises as a Python exception at the call site.
"""

import ast
import json
import os
import socket
import struct
import sys
import threading
import traceback
import queue as _queue

try:
    import ctypes  # used for cooperative cancellation
except Exception:  # pragma: no cover - ctypes is present on CPython
    ctypes = None

_OUTPUT_CAP_DEFAULT = 50 * 1024

_sock = None
_write_lock = threading.Lock()
_pending = {}
_pending_lock = threading.Lock()
_id_lock = threading.Lock()
_next_id = 0
_ns = {}
_current_exec = None
_worker_tid = None
_exec_queue = _queue.Queue()
_exc_types = {}


# --- Framing -------------------------------------------------------------

def _recv_exact(n):
    buf = b""
    while len(buf) < n:
        chunk = _sock.recv(n - len(buf))
        if not chunk:
            raise ConnectionError("control socket closed")
        buf += chunk
    return buf


def _read_frame():
    (length,) = struct.unpack(">I", _recv_exact(4))
    return json.loads(_recv_exact(length))


def _write_frame(msg):
    payload = json.dumps(msg).encode("utf-8")
    header = struct.pack(">I", len(payload))
    with _write_lock:
        _sock.sendall(header + payload)


def _notify(method, params):
    _write_frame({"jsonrpc": "2.0", "method": method, "params": params})


def _alloc_id():
    global _next_id
    with _id_lock:
        _next_id += 1
        return _next_id


# --- Capability proxies --------------------------------------------------

class CapabilityError(Exception):
    """Base class for any error returned by a host capability."""


def _exc_for(etype):
    cls = _exc_types.get(etype)
    if cls is None:
        cls = type(str(etype), (CapabilityError,), {})
        _exc_types[etype] = cls
    return cls


def _invoke(name, args):
    msg_id = _alloc_id()
    reply_q = _queue.Queue(maxsize=1)
    with _pending_lock:
        _pending[msg_id] = reply_q
    try:
        _write_frame({
            "jsonrpc": "2.0",
            "id": msg_id,
            "method": "cap.call",
            "params": {"exec_id": _current_exec, "name": name, "args": args},
        })
        resp = reply_q.get()
    finally:
        with _pending_lock:
            _pending.pop(msg_id, None)

    err = resp.get("error")
    if err is not None:
        etype = "CapabilityError"
        data = err.get("data") or {}
        if isinstance(data, dict) and data.get("type"):
            etype = data["type"]
        raise _exc_for(etype)(err.get("message", ""))
    return resp.get("result")


def _make_proxy(name, param_names):
    def proxy(*args, **kwargs):
        call_args = dict(kwargs)
        for i, value in enumerate(args):
            if i >= len(param_names):
                raise TypeError(
                    "%s() takes at most %d positional argument(s)" % (name, len(param_names))
                )
            call_args[param_names[i]] = value
        return _invoke(name, call_args)

    proxy.__name__ = name
    return proxy


# --- Stdout capture ------------------------------------------------------

class _StreamWriter:
    def __init__(self, exec_id, cap):
        self._exec_id = exec_id
        self._cap = cap
        self._sent = 0
        self.truncated = False

    def write(self, s):
        if not s:
            return
        if self._sent >= self._cap:
            self.truncated = True
            return
        remaining = self._cap - self._sent
        chunk = s if len(s) <= remaining else s[:remaining]
        if len(s) > remaining:
            self.truncated = True
        self._sent += len(chunk)
        _notify("out.stdout", {"exec_id": self._exec_id, "chunk": chunk})

    def flush(self):
        pass


# --- Execution -----------------------------------------------------------

def _exec_with_result(code):
    """Execute code in the persistent namespace. If the final statement is an
    expression, return repr(value); otherwise return None."""
    tree = ast.parse(code, "<cell>", "exec")
    if tree.body and isinstance(tree.body[-1], ast.Expr):
        last = tree.body.pop()
        if tree.body:
            exec(compile(tree, "<cell>", "exec"), _ns)
        value = eval(compile(ast.Expression(last.value), "<cell>", "eval"), _ns)
        return None if value is None else repr(value)
    exec(compile(tree, "<cell>", "exec"), _ns)
    return None


def _run_code(exec_id, code, output_cap):
    global _current_exec
    _current_exec = exec_id
    writer = _StreamWriter(exec_id, output_cap or _OUTPUT_CAP_DEFAULT)
    saved_out, saved_err = sys.stdout, sys.stderr
    sys.stdout = writer
    sys.stderr = writer
    try:
        result_text = _exec_with_result(code)
    except KeyboardInterrupt:
        sys.stdout, sys.stderr = saved_out, saved_err
        _notify("exec.error", {
            "exec_id": exec_id, "type": "KernelCancelled",
            "message": "execution cancelled", "traceback": "",
        })
    except BaseException as exc:  # report any failure back to the host
        sys.stdout, sys.stderr = saved_out, saved_err
        _notify("exec.error", {
            "exec_id": exec_id, "type": type(exc).__name__,
            "message": str(exc), "traceback": traceback.format_exc(),
        })
    else:
        sys.stdout, sys.stderr = saved_out, saved_err
        if result_text is not None:
            _notify("exec.result", {"exec_id": exec_id, "text": result_text, "mime": "text/plain"})
    finally:
        sys.stdout, sys.stderr = saved_out, saved_err
        _current_exec = None
    _notify("exec.done", {"exec_id": exec_id, "output_truncated": writer.truncated})


def _worker():
    global _worker_tid
    _worker_tid = threading.get_ident()
    while True:
        item = _exec_queue.get()
        if item is None:
            return
        exec_id, code, output_cap = item
        _run_code(exec_id, code, output_cap)


def _cancel_worker():
    if ctypes is None or _worker_tid is None:
        return
    ctypes.pythonapi.PyThreadState_SetAsyncExc(
        ctypes.c_long(_worker_tid), ctypes.py_object(KeyboardInterrupt)
    )


# --- Reader loop ---------------------------------------------------------

def _reader():
    while True:
        try:
            msg = _read_frame()
        except Exception:
            return  # socket closed: let the process exit
        method = msg.get("method")
        if method == "exec.submit":
            params = msg.get("params") or {}
            limits = params.get("limits") or {}
            _exec_queue.put((
                params.get("exec_id"),
                params.get("code", ""),
                limits.get("output_cap", 0),
            ))
        elif method == "exec.cancel":
            _cancel_worker()
        elif method is None and "id" in msg:
            with _pending_lock:
                reply_q = _pending.get(msg["id"])
            if reply_q is not None:
                reply_q.put(msg)


def _main():
    global _sock
    fd_env = os.environ.get("PYKERNEL_SOCKET_FD")
    if not fd_env:
        sys.stderr.write("pykernel: PYKERNEL_SOCKET_FD not set\n")
        sys.exit(1)

    # The host passes the control socket as an inherited fd (no filesystem path,
    # so no sun_path length limit).
    _sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM, fileno=int(fd_env))
    _sock.setblocking(True)

    for spec in json.loads(os.environ.get("PYKERNEL_CAPABILITIES", "[]")):
        param_names = [p.get("name") for p in (spec.get("params") or [])]
        _ns[spec["name"]] = _make_proxy(spec["name"], param_names)

    threading.Thread(target=_worker, daemon=True).start()
    _reader()


if __name__ == "__main__":
    _main()
