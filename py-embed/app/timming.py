import functools
import inspect
import time
import typing


def get_full_method_name(method: typing.Callable) -> str:
    if hasattr(method, '__self__'):
        return method.__self__.__class__.__name__ + "." + method.__name__
    return method.__repr__()


def to_readable_number(total: float) -> tuple[float, str]:
    if total > 1_000_000_000:
        return total / 1_000_000_000, "s"

    if total > 1_000_000:
        return total / 1_000_000, "ms"

    if total > 1000:
        return total / 1000, "μs"

    return total, "ns"


class Timer:
    def __init__(self, description: str):
        self.start_time = 0
        self.description = description

    def start(self):
        self.start_time = time.time_ns()

    def stop(self):
        total, unit = to_readable_number(time.time_ns() - self.start_time)
        print(f"'{self.description}' took {total:.2f} {unit}")


def timeit(method: typing.Callable):
    if inspect.iscoroutinefunction(method):
        @functools.wraps(method)
        async def wrap(*args, **kwargs):
            start = time.time_ns()
            result = await method(*args, **kwargs)
            total, unit = to_readable_number(time.time_ns() - start)
            method_name = get_full_method_name(method)
            print(f"Method '{method_name}' took {total:.2f} {unit}")
            return result

        return wrap

    @functools.wraps(method)
    def wrap(*args, **kwargs):
        start = time.time_ns()
        result = method(*args, **kwargs)
        total, unit = to_readable_number(time.time_ns() - start)
        method_name = get_full_method_name(method)
        print(f"Method '{method_name}' took {total:.2f} {unit}")
        return result

    return wrap
