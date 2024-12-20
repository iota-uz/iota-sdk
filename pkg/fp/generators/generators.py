def compose_test(n: int) -> str:
	funcs = ["add1", "double"]
	fns = ", ".join(funcs[i % 2] for i in range(n))

	expect = 0
	for i in range(n-1, -1, -1):
		if i % 2 == 0:
			expect += 1
		else:
			expect *= 2

	return f"""
func TestCompose{n}_Example(t *testing.T) {{
    res := Compose{n}({fns})(0)
    if res != {expect} {{
        t.Error("Should perform right-to-left function composition of {n} functions. Received:", res)
    }}
}}"""


def pipe_test(n: int) -> str:
	funcs = ["add1", "double"]
	fns = ", ".join(funcs[i % 2] for i in range(n))

	expect = 0
	for i in range(n):
		if i % 2 == 0:
			expect += 1
		else:
			expect *= 2

	return f"""
func TestPipe{n}_Example(t *testing.T) {{
	res := Pipe{n}({fns})(0)
	if res != {expect} {{
		t.Error("Should perform left-to-right function composition of {n} functions. Received:", res)
	}}
}}"""


def curry_test(n: int) -> str:
    vars = [f"t{i+1}" for i in range(n)]

    params = ", ".join(vars)
    result = " + ".join(vars)
    call   = "".join(f"({i})" for i in range(1, n+1))
    res    = sum(range(1, n+1))

    return f"""
func TestCurry{n}_Example(t *testing.T) {{
    function := func({params} int) int {{ return {result} }}
    curriedF := Curry{n}(function)

    res := curriedF{call}

    if res != {res} {{
        t.Error("Should allow to curry a sum. Received:", res)
    }}
}}"""


def pipe_func(n: int) -> str:
    gen = "[" + ", ".join(f"T{i+1}" for i in range(n)) + ", R any]"
    fns = ", ".join(f"fn{i} func(T{i}) T{i+1}" for i in range(1, n)) + f", fn{n} func(T{n}) R"
    cal = "".join(f"fn{i}(" for i in range(n, 0, -1)) + "t1" + ")" * n

    return f"""// Performs left-to-right function composition of {n} functions
func Pipe{n}{gen}({fns}) func(T1) R  {{
    return func(t1 T1) R {{
        return {cal}
    }}
}}"""


def compose_func(n: int) -> str:
    gen = "[" + ", ".join(f"T{i+1}" for i in range(n)) + ", R any]"
    fns = f"fn{n} func(T{n}) R, " + ", ".join(f"fn{i} func(T{i}) T{i+1}" for i in range(n-1, 0, -1))
    cal = "".join(f"fn{i}(" for i in range(n, 0, -1)) + "t1" + ")" * n

    return f"""// Performs right-to-left function composition of {n} functions
func Compose{n}{gen}({fns}) func(T1) R  {{
    return func(t1 T1) R {{
        return {cal}
    }}
}}"""


def curry_func(n: int) -> str:
    gen = ", ".join(f"T{i+1}" for i in range(n))
    ret = " ".join(f"func(T{i+1})" for i in range(n)) + " R"

    line = lambda x: x * "\t" + f"return func(t{x} T{x}) " + " ".join(f"func(T{i+1})" for i in range(x, n)) + " R {"

    all_returns = "\n".join(line(i+1) for i in range(n))

    function = (n+1) * "\t" + "return fn(" + ", ".join(f"t{i+1}" for i in range(n)) + ")"

    all_closes = "\n".join(i * "\t" + "}" for i in range(n, 0, -1))

    return f"""// Allow to transform a function that receives {n} params in a sequence of unary functions
func Curry{n}[{gen}, R any](fn func({gen}) R) {ret} {{
{all_returns}
{function}
{all_closes}
}}
"""