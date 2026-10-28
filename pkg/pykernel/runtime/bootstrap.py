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
import asyncio
import inspect
import json
import os
import socket
import struct
import sys
import threading
import traceback
import types
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
_loop = None


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


_PY_TYPES = {"str": str, "int": int, "float": float, "bool": bool, "dict": dict, "list": list}


def _signature(params):
    out = []
    for p in params:
        annotation = _PY_TYPES.get(p.get("type") or "", inspect.Parameter.empty)
        default = inspect.Parameter.empty if p.get("required") else None
        out.append(inspect.Parameter(
            p["name"], inspect.Parameter.KEYWORD_ONLY, default=default, annotation=annotation,
        ))
    return inspect.Signature(out)


def _make_proxy(spec):
    name = spec["name"]
    params = spec.get("params") or []
    param_names = [p.get("name") for p in params]

    def call(args, kwargs):
        call_args = dict(kwargs)
        for i, value in enumerate(args):
            if i >= len(param_names):
                raise TypeError(
                    "%s() takes at most %d positional argument(s)" % (name, len(param_names))
                )
            call_args[param_names[i]] = value
        return call_args

    if spec.get("async"):
        async def proxy(*args, **kwargs):
            call_args = call(args, kwargs)
            return await asyncio.get_running_loop().run_in_executor(None, _invoke, name, call_args)
    else:
        def proxy(*args, **kwargs):
            return _invoke(name, call(args, kwargs))

    proxy.__name__ = name
    proxy.__doc__ = spec.get("doc") or None
    if spec.get("namespace"):
        proxy.__signature__ = _signature(params)
    return proxy


# --- Stdout capture ------------------------------------------------------

class _Budget:
    """Output cap shared by stdout, stderr and the result of one exec."""

    def __init__(self, cap):
        self.cap = cap
        self.sent = 0


class _StreamWriter:
    def __init__(self, exec_id, budget, method):
        self._exec_id = exec_id
        self._budget = budget
        self._method = method
        self.truncated = False

    def write(self, s):
        # The cap is measured in UTF-8 bytes (matching the Go-side OutputCap),
        # not Python code points, so we encode and budget in bytes and never
        # split a multibyte sequence when truncating. A file-like write must
        # return the number of characters written, so every path returns the
        # original code-point length of the input.
        n = len(s)
        if not s:
            return 0
        budget = self._budget
        if budget.sent >= budget.cap:
            self.truncated = True
            return n
        encoded = s.encode("utf-8")
        remaining = budget.cap - budget.sent
        if len(encoded) <= remaining:
            budget.sent += len(encoded)
            _notify(self._method, {"exec_id": self._exec_id, "chunk": s})
            return n
        # Truncate on a valid UTF-8 boundary: take the first `remaining` bytes,
        # then drop any partial trailing char via errors='ignore'.
        chunk = encoded[:remaining].decode("utf-8", errors="ignore")
        self.truncated = True
        budget.sent = budget.cap
        if chunk:
            _notify(self._method, {"exec_id": self._exec_id, "chunk": chunk})
        return n

    def flush(self):
        pass


# --- Execution -----------------------------------------------------------

_COMPILE_FLAGS = ast.PyCF_ALLOW_TOP_LEVEL_AWAIT


def _run(code_obj):
    """Evaluate a compiled cell; a cell using top-level await compiles to a
    coroutine, which runs on the kernel's persistent event loop."""
    global _loop
    value = eval(code_obj, _ns)
    if code_obj.co_flags & inspect.CO_COROUTINE:
        if _loop is None:
            _loop = asyncio.new_event_loop()
        value = _loop.run_until_complete(value)
    return value


def _exec_with_result(code):
    """Execute code in the persistent namespace. If the final statement is an
    expression, return repr(value); otherwise return None."""
    tree = ast.parse(code, "<cell>", "exec")
    if tree.body and isinstance(tree.body[-1], ast.Expr):
        last = tree.body.pop()
        if tree.body:
            _run(compile(tree, "<cell>", "exec", flags=_COMPILE_FLAGS))
        value = _run(compile(ast.Expression(last.value), "<cell>", "eval", flags=_COMPILE_FLAGS))
        return None if value is None else repr(value)
    _run(compile(tree, "<cell>", "exec", flags=_COMPILE_FLAGS))
    return None


def _deliver(method, params):
    """Send a notification even if a late cancellation lands meanwhile. Once
    _current_exec is cleared no further interrupt can be injected, so at most
    one retry happens; a duplicated frame is ignored by the host."""
    while True:
        try:
            _notify(method, params)
            return
        except KeyboardInterrupt:
            continue


def _run_code(exec_id, code, output_cap):
    global _current_exec
    _current_exec = exec_id
    budget = _Budget(output_cap or _OUTPUT_CAP_DEFAULT)
    writer = _StreamWriter(exec_id, budget, "out.stdout")
    err_writer = _StreamWriter(exec_id, budget, "out.stderr")
    saved_out, saved_err = sys.stdout, sys.stderr
    sys.stdout = writer
    sys.stderr = err_writer
    error = None
    result_text = None
    try:
        try:
            result_text = _exec_with_result(code)
        except KeyboardInterrupt:
            error = ("KernelCancelled", "execution cancelled", "")
        except BaseException as exc:  # report any failure back to the host
            error = (type(exc).__name__, str(exc), traceback.format_exc())
    except KeyboardInterrupt:
        # The cancellation raced with the end of the cell.
        if error is None:
            error = ("KernelCancelled", "execution cancelled", "")
    finally:
        _current_exec = None
        sys.stdout, sys.stderr = saved_out, saved_err

    if error is not None:
        _deliver("exec.error", {
            "exec_id": exec_id, "type": error[0], "message": error[1], "traceback": error[2],
        })
    elif result_text is not None:
        # Budget the result against the SAME byte cap as stdout so a huge
        # repr can't exceed the bridge frame cap or balloon memory. Truncate
        # on a UTF-8 boundary and flag the exec as truncated.
        remaining = budget.cap - budget.sent
        encoded = result_text.encode("utf-8")
        if remaining <= 0:
            # Budget exhausted by stdout: elide the result entirely.
            result_text = "… [truncated]"
            writer.truncated = True
        elif len(encoded) > remaining:
            result_text = encoded[:remaining].decode("utf-8", errors="ignore") + "… [truncated]"
            budget.sent = budget.cap
            writer.truncated = True
        else:
            budget.sent += len(encoded)
        _deliver("exec.result", {"exec_id": exec_id, "text": result_text, "mime": "text/plain"})
    _deliver("exec.done", {
        "exec_id": exec_id,
        "output_truncated": writer.truncated or err_writer.truncated,
        "stdout_truncated": writer.truncated,
        "stderr_truncated": err_writer.truncated,
    })


def _worker():
    global _worker_tid
    _worker_tid = threading.get_ident()
    while True:
        try:
            item = _exec_queue.get()
            if item is None:
                return
            exec_id, code, output_cap = item
            _run_code(exec_id, code, output_cap)
        except KeyboardInterrupt:
            # A cancellation can be delivered late (after the target exec
            # finished) or while the worker is idle in _exec_queue.get(). Such a
            # stray async exception must NOT terminate the only execution
            # thread — swallow it and keep serving the queue. The in-exec case
            # is handled by _run_code's own KeyboardInterrupt handler.
            continue


def _cancel_worker(target_exec_id):
    if ctypes is None or _worker_tid is None:
        return
    # Best-effort gate: only interrupt when the requested exec is the one
    # actually running. A tiny TOCTOU window remains but is far safer than
    # interrupting unconditionally (which could cancel a different exec or land
    # in the idle worker loop).
    current = _current_exec
    if current is None or current != target_exec_id:
        return
    # PyThreadState_SetAsyncExc returns the number of thread states modified.
    # 0 = tid not found (nothing to do). >1 = it hit more than the intended
    # thread, which per CPython requires an immediate revert by calling again
    # with a NULL exception, and is treated as a failed cancellation.
    res = ctypes.pythonapi.PyThreadState_SetAsyncExc(
        ctypes.c_long(_worker_tid), ctypes.py_object(KeyboardInterrupt)
    )
    if res > 1:
        ctypes.pythonapi.PyThreadState_SetAsyncExc(ctypes.c_long(_worker_tid), None)


# --- Reader loop ---------------------------------------------------------

def _fail_pending(message):
    """Unblock every waiting _invoke when the control socket dies, so capability
    calls raise instead of wedging the worker thread forever."""
    sentinel = {"error": {"message": message, "data": {"type": "KernelDisconnected"}}}
    with _pending_lock:
        pending = list(_pending.values())
        _pending.clear()
    for reply_q in pending:
        # Reply queues are maxsize=1; clear any stale item, then push without
        # blocking so a full queue cannot deadlock the reader's teardown.
        try:
            reply_q.put_nowait(sentinel)
        except _queue.Full:
            try:
                reply_q.get_nowait()
            except _queue.Empty:
                pass
            try:
                reply_q.put_nowait(sentinel)
            except _queue.Full:
                pass


def _reader():
    try:
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
                params = msg.get("params") or {}
                _cancel_worker(params.get("exec_id"))
            elif method is None and "id" in msg:
                with _pending_lock:
                    reply_q = _pending.get(msg["id"])
                if reply_q is not None:
                    # Deliver non-blockingly: the reply queue is maxsize=1, so a
                    # duplicate/late response (or a race with _fail_pending) must
                    # not wedge the reader. Drop/replace any stale item.
                    try:
                        reply_q.put_nowait(msg)
                    except _queue.Full:
                        try:
                            reply_q.get_nowait()
                        except _queue.Empty:
                            pass
                        try:
                            reply_q.put_nowait(msg)
                        except _queue.Full:
                            pass
    finally:
        _fail_pending("control socket closed")


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
        proxy = _make_proxy(spec)
        namespace = spec.get("namespace")
        if namespace:
            _ns.setdefault(namespace, types.SimpleNamespace())
            setattr(_ns[namespace], spec["name"], proxy)
        else:
            _ns[spec["name"]] = proxy

    threading.Thread(target=_worker, daemon=True).start()
    _reader()


if __name__ == "__main__":
    _main()
