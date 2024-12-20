package fp

// Performs left-to-right function composition of two functions
func Pipe2[T1, T2, R any](fn1 func(T1) T2, fn2 func(T2) R) func(T1) R {
	return func(t1 T1) R {
		return fn2(fn1(t1))
	}
}

// Performs left-to-right function composition of three functions
func Pipe3[T1, T2, T3, R any](fn1 func(T1) T2, fn2 func(T2) T3, fn3 func(T3) R) func(T1) R {
	return func(t1 T1) R {
		return fn3(fn2(fn1(t1)))
	}
}

// Performs left-to-right function composition of four functions
func Pipe4[T1, T2, T3, T4, R any](fn1 func(T1) T2, fn2 func(T2) T3, fn3 func(T3) T4, fn4 func(T4) R) func(T1) R {
	return func(t1 T1) R {
		return fn4(fn3(fn2(fn1(t1))))
	}
}

// Performs left-to-right function composition of five functions
func Pipe5[T1, T2, T3, T4, T5, R any](fn1 func(T1) T2, fn2 func(T2) T3, fn3 func(T3) T4, fn4 func(T4) T5, fn5 func(T5) R) func(T1) R {
	return func(t1 T1) R {
		return fn5(fn4(fn3(fn2(fn1(t1)))))
	}
}

// Performs left-to-right function composition of 6 functions
func Pipe6[T1, T2, T3, T4, T5, T6, R any](fn1 func(T1) T2, fn2 func(T2) T3, fn3 func(T3) T4, fn4 func(T4) T5, fn5 func(T5) T6, fn6 func(T6) R) func(T1) R {
	return func(t1 T1) R {
		return fn6(fn5(fn4(fn3(fn2(fn1(t1))))))
	}
}

// Performs left-to-right function composition of 7 functions
func Pipe7[T1, T2, T3, T4, T5, T6, T7, R any](fn1 func(T1) T2, fn2 func(T2) T3, fn3 func(T3) T4, fn4 func(T4) T5, fn5 func(T5) T6, fn6 func(T6) T7, fn7 func(T7) R) func(T1) R {
	return func(t1 T1) R {
		return fn7(fn6(fn5(fn4(fn3(fn2(fn1(t1)))))))
	}
}

// Performs left-to-right function composition of 8 functions
func Pipe8[T1, T2, T3, T4, T5, T6, T7, T8, R any](fn1 func(T1) T2, fn2 func(T2) T3, fn3 func(T3) T4, fn4 func(T4) T5, fn5 func(T5) T6, fn6 func(T6) T7, fn7 func(T7) T8, fn8 func(T8) R) func(T1) R {
	return func(t1 T1) R {
		return fn8(fn7(fn6(fn5(fn4(fn3(fn2(fn1(t1))))))))
	}
}

// Performs left-to-right function composition of 9 functions
func Pipe9[T1, T2, T3, T4, T5, T6, T7, T8, T9, R any](fn1 func(T1) T2, fn2 func(T2) T3, fn3 func(T3) T4, fn4 func(T4) T5, fn5 func(T5) T6, fn6 func(T6) T7, fn7 func(T7) T8, fn8 func(T8) T9, fn9 func(T9) R) func(T1) R {
	return func(t1 T1) R {
		return fn9(fn8(fn7(fn6(fn5(fn4(fn3(fn2(fn1(t1)))))))))
	}
}

// Performs left-to-right function composition of 10 functions
func Pipe10[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, R any](fn1 func(T1) T2, fn2 func(T2) T3, fn3 func(T3) T4, fn4 func(T4) T5, fn5 func(T5) T6, fn6 func(T6) T7, fn7 func(T7) T8, fn8 func(T8) T9, fn9 func(T9) T10, fn10 func(T10) R) func(T1) R {
	return func(t1 T1) R {
		return fn10(fn9(fn8(fn7(fn6(fn5(fn4(fn3(fn2(fn1(t1))))))))))
	}
}

// Performs left-to-right function composition of 11 functions
func Pipe11[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, R any](fn1 func(T1) T2, fn2 func(T2) T3, fn3 func(T3) T4, fn4 func(T4) T5, fn5 func(T5) T6, fn6 func(T6) T7, fn7 func(T7) T8, fn8 func(T8) T9, fn9 func(T9) T10, fn10 func(T10) T11, fn11 func(T11) R) func(T1) R {
	return func(t1 T1) R {
		return fn11(fn10(fn9(fn8(fn7(fn6(fn5(fn4(fn3(fn2(fn1(t1)))))))))))
	}
}

// Performs left-to-right function composition of 12 functions
func Pipe12[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12, R any](fn1 func(T1) T2, fn2 func(T2) T3, fn3 func(T3) T4, fn4 func(T4) T5, fn5 func(T5) T6, fn6 func(T6) T7, fn7 func(T7) T8, fn8 func(T8) T9, fn9 func(T9) T10, fn10 func(T10) T11, fn11 func(T11) T12, fn12 func(T12) R) func(T1) R {
	return func(t1 T1) R {
		return fn12(fn11(fn10(fn9(fn8(fn7(fn6(fn5(fn4(fn3(fn2(fn1(t1))))))))))))
	}
}

// Performs left-to-right function composition of 13 functions
func Pipe13[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12, T13, R any](fn1 func(T1) T2, fn2 func(T2) T3, fn3 func(T3) T4, fn4 func(T4) T5, fn5 func(T5) T6, fn6 func(T6) T7, fn7 func(T7) T8, fn8 func(T8) T9, fn9 func(T9) T10, fn10 func(T10) T11, fn11 func(T11) T12, fn12 func(T12) T13, fn13 func(T13) R) func(T1) R {
	return func(t1 T1) R {
		return fn13(fn12(fn11(fn10(fn9(fn8(fn7(fn6(fn5(fn4(fn3(fn2(fn1(t1)))))))))))))
	}
}

// Performs left-to-right function composition of 14 functions
func Pipe14[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12, T13, T14, R any](fn1 func(T1) T2, fn2 func(T2) T3, fn3 func(T3) T4, fn4 func(T4) T5, fn5 func(T5) T6, fn6 func(T6) T7, fn7 func(T7) T8, fn8 func(T8) T9, fn9 func(T9) T10, fn10 func(T10) T11, fn11 func(T11) T12, fn12 func(T12) T13, fn13 func(T13) T14, fn14 func(T14) R) func(T1) R {
	return func(t1 T1) R {
		return fn14(fn13(fn12(fn11(fn10(fn9(fn8(fn7(fn6(fn5(fn4(fn3(fn2(fn1(t1))))))))))))))
	}
}

// Performs left-to-right function composition of 15 functions
func Pipe15[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12, T13, T14, T15, R any](fn1 func(T1) T2, fn2 func(T2) T3, fn3 func(T3) T4, fn4 func(T4) T5, fn5 func(T5) T6, fn6 func(T6) T7, fn7 func(T7) T8, fn8 func(T8) T9, fn9 func(T9) T10, fn10 func(T10) T11, fn11 func(T11) T12, fn12 func(T12) T13, fn13 func(T13) T14, fn14 func(T14) T15, fn15 func(T15) R) func(T1) R {
	return func(t1 T1) R {
		return fn15(fn14(fn13(fn12(fn11(fn10(fn9(fn8(fn7(fn6(fn5(fn4(fn3(fn2(fn1(t1)))))))))))))))
	}
}

// Performs left-to-right function composition of 16 functions
func Pipe16[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12, T13, T14, T15, T16, R any](fn1 func(T1) T2, fn2 func(T2) T3, fn3 func(T3) T4, fn4 func(T4) T5, fn5 func(T5) T6, fn6 func(T6) T7, fn7 func(T7) T8, fn8 func(T8) T9, fn9 func(T9) T10, fn10 func(T10) T11, fn11 func(T11) T12, fn12 func(T12) T13, fn13 func(T13) T14, fn14 func(T14) T15, fn15 func(T15) T16, fn16 func(T16) R) func(T1) R {
	return func(t1 T1) R {
		return fn16(fn15(fn14(fn13(fn12(fn11(fn10(fn9(fn8(fn7(fn6(fn5(fn4(fn3(fn2(fn1(t1))))))))))))))))
	}
}
