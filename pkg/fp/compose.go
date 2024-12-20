package fp

// Performs right-to-left function composition of two functions
func Compose2[T1, T2, R any](fn2 func(T2) R, fn1 func(T1) T2) func(T1) R {
	return func(t1 T1) R {
		return fn2(fn1(t1))
	}
}

// Performs right-to-left function composition of three functions
func Compose3[T1, T2, T3, R any](fn3 func(T3) R, fn2 func(T2) T3, fn1 func(T1) T2) func(T1) R {
	return func(t1 T1) R {
		return fn3(fn2(fn1(t1)))
	}
}

// Performs right-to-left function composition of four functions
func Compose4[T1, T2, T3, T4, R any](fn4 func(T4) R, fn3 func(T3) T4, fn2 func(T2) T3, fn1 func(T1) T2) func(T1) R {
	return func(t1 T1) R {
		return fn4(fn3(fn2(fn1(t1))))
	}
}

// Performs right-to-left function composition of 5 functions
func Compose5[T1, T2, T3, T4, T5, R any](fn5 func(T5) R, fn4 func(T4) T5, fn3 func(T3) T4, fn2 func(T2) T3, fn1 func(T1) T2) func(T1) R {
	return func(t1 T1) R {
		return fn5(fn4(fn3(fn2(fn1(t1)))))
	}
}

// Performs right-to-left function composition of 6 functions
func Compose6[T1, T2, T3, T4, T5, T6, R any](fn6 func(T6) R, fn5 func(T5) T6, fn4 func(T4) T5, fn3 func(T3) T4, fn2 func(T2) T3, fn1 func(T1) T2) func(T1) R {
	return func(t1 T1) R {
		return fn6(fn5(fn4(fn3(fn2(fn1(t1))))))
	}
}

// Performs right-to-left function composition of 7 functions
func Compose7[T1, T2, T3, T4, T5, T6, T7, R any](fn7 func(T7) R, fn6 func(T6) T7, fn5 func(T5) T6, fn4 func(T4) T5, fn3 func(T3) T4, fn2 func(T2) T3, fn1 func(T1) T2) func(T1) R {
	return func(t1 T1) R {
		return fn7(fn6(fn5(fn4(fn3(fn2(fn1(t1)))))))
	}
}

// Performs right-to-left function composition of 8 functions
func Compose8[T1, T2, T3, T4, T5, T6, T7, T8, R any](fn8 func(T8) R, fn7 func(T7) T8, fn6 func(T6) T7, fn5 func(T5) T6, fn4 func(T4) T5, fn3 func(T3) T4, fn2 func(T2) T3, fn1 func(T1) T2) func(T1) R {
	return func(t1 T1) R {
		return fn8(fn7(fn6(fn5(fn4(fn3(fn2(fn1(t1))))))))
	}
}

// Performs right-to-left function composition of 9 functions
func Compose9[T1, T2, T3, T4, T5, T6, T7, T8, T9, R any](fn9 func(T9) R, fn8 func(T8) T9, fn7 func(T7) T8, fn6 func(T6) T7, fn5 func(T5) T6, fn4 func(T4) T5, fn3 func(T3) T4, fn2 func(T2) T3, fn1 func(T1) T2) func(T1) R {
	return func(t1 T1) R {
		return fn9(fn8(fn7(fn6(fn5(fn4(fn3(fn2(fn1(t1)))))))))
	}
}

// Performs right-to-left function composition of 10 functions
func Compose10[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, R any](fn10 func(T10) R, fn9 func(T9) T10, fn8 func(T8) T9, fn7 func(T7) T8, fn6 func(T6) T7, fn5 func(T5) T6, fn4 func(T4) T5, fn3 func(T3) T4, fn2 func(T2) T3, fn1 func(T1) T2) func(T1) R {
	return func(t1 T1) R {
		return fn10(fn9(fn8(fn7(fn6(fn5(fn4(fn3(fn2(fn1(t1))))))))))
	}
}

// Performs right-to-left function composition of 11 functions
func Compose11[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, R any](fn11 func(T11) R, fn10 func(T10) T11, fn9 func(T9) T10, fn8 func(T8) T9, fn7 func(T7) T8, fn6 func(T6) T7, fn5 func(T5) T6, fn4 func(T4) T5, fn3 func(T3) T4, fn2 func(T2) T3, fn1 func(T1) T2) func(T1) R {
	return func(t1 T1) R {
		return fn11(fn10(fn9(fn8(fn7(fn6(fn5(fn4(fn3(fn2(fn1(t1)))))))))))
	}
}

// Performs right-to-left function composition of 12 functions
func Compose12[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12, R any](fn12 func(T12) R, fn11 func(T11) T12, fn10 func(T10) T11, fn9 func(T9) T10, fn8 func(T8) T9, fn7 func(T7) T8, fn6 func(T6) T7, fn5 func(T5) T6, fn4 func(T4) T5, fn3 func(T3) T4, fn2 func(T2) T3, fn1 func(T1) T2) func(T1) R {
	return func(t1 T1) R {
		return fn12(fn11(fn10(fn9(fn8(fn7(fn6(fn5(fn4(fn3(fn2(fn1(t1))))))))))))
	}
}

// Performs right-to-left function composition of 13 functions
func Compose13[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12, T13, R any](fn13 func(T13) R, fn12 func(T12) T13, fn11 func(T11) T12, fn10 func(T10) T11, fn9 func(T9) T10, fn8 func(T8) T9, fn7 func(T7) T8, fn6 func(T6) T7, fn5 func(T5) T6, fn4 func(T4) T5, fn3 func(T3) T4, fn2 func(T2) T3, fn1 func(T1) T2) func(T1) R {
	return func(t1 T1) R {
		return fn13(fn12(fn11(fn10(fn9(fn8(fn7(fn6(fn5(fn4(fn3(fn2(fn1(t1)))))))))))))
	}
}

// Performs right-to-left function composition of 14 functions
func Compose14[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12, T13, T14, R any](fn14 func(T14) R, fn13 func(T13) T14, fn12 func(T12) T13, fn11 func(T11) T12, fn10 func(T10) T11, fn9 func(T9) T10, fn8 func(T8) T9, fn7 func(T7) T8, fn6 func(T6) T7, fn5 func(T5) T6, fn4 func(T4) T5, fn3 func(T3) T4, fn2 func(T2) T3, fn1 func(T1) T2) func(T1) R {
	return func(t1 T1) R {
		return fn14(fn13(fn12(fn11(fn10(fn9(fn8(fn7(fn6(fn5(fn4(fn3(fn2(fn1(t1))))))))))))))
	}
}

// Performs right-to-left function composition of 15 functions
func Compose15[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12, T13, T14, T15, R any](fn15 func(T15) R, fn14 func(T14) T15, fn13 func(T13) T14, fn12 func(T12) T13, fn11 func(T11) T12, fn10 func(T10) T11, fn9 func(T9) T10, fn8 func(T8) T9, fn7 func(T7) T8, fn6 func(T6) T7, fn5 func(T5) T6, fn4 func(T4) T5, fn3 func(T3) T4, fn2 func(T2) T3, fn1 func(T1) T2) func(T1) R {
	return func(t1 T1) R {
		return fn15(fn14(fn13(fn12(fn11(fn10(fn9(fn8(fn7(fn6(fn5(fn4(fn3(fn2(fn1(t1)))))))))))))))
	}
}

// Performs right-to-left function composition of 16 functions
func Compose16[T1, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12, T13, T14, T15, T16, R any](fn16 func(T16) R, fn15 func(T15) T16, fn14 func(T14) T15, fn13 func(T13) T14, fn12 func(T12) T13, fn11 func(T11) T12, fn10 func(T10) T11, fn9 func(T9) T10, fn8 func(T8) T9, fn7 func(T7) T8, fn6 func(T6) T7, fn5 func(T5) T6, fn4 func(T4) T5, fn3 func(T3) T4, fn2 func(T2) T3, fn1 func(T1) T2) func(T1) R {
	return func(t1 T1) R {
		return fn16(fn15(fn14(fn13(fn12(fn11(fn10(fn9(fn8(fn7(fn6(fn5(fn4(fn3(fn2(fn1(t1))))))))))))))))
	}
}
