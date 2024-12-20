package either

import (
	"strconv"
	"testing"

	opt "github.com/repeale/fp-go/option"
)

func isOdd(n int) bool {
	return n%2 != 0
}

func onLeft() string {
	return "myOnLeftCallbackError"
}

func convertToIntAndAdd1(x string) int {
	intVar, _ := strconv.Atoi(x)
	return 1 + intVar
}

func convertToIntAndAdd2(x string) int {
	intVar, _ := strconv.Atoi(x)
	return 2 + intVar
}

func TestLeft(t *testing.T) {
	leftVal := "myError"
	res := Left[string, string](leftVal)
	if res.isLeft != true {
		t.Error("Left should return an Either struct with isLeft set to true . Received:", res.isLeft)
	}
	if res.left != leftVal {
		t.Error("Left should return an Either struct with the left value stored . Received:", res.left)
	}
}

func TestRight(t *testing.T) {
	rightVal := "myVal"
	res := Right[string](rightVal)
	if res.isLeft == true {
		t.Error("Right should return an Either struct with isLeft set to false . Received:", res.isLeft)
	}
	if res.right != rightVal {
		t.Error("Right should return an Either struct with the right value stored . Received:", res.right)
	}
}

func TestIsLeft_Left(t *testing.T) {
	res := IsLeft(Left[string, string]("myVal"))
	if res != true {
		t.Error("IsLeft should return true. Received:", res)
	}
}
func TestIsLeft_Right(t *testing.T) {
	res := IsLeft(Right[string]("myVal"))
	if res != false {
		t.Error("IsLeft should return false. Received:", res)
	}
}

func TestIsRight_Left(t *testing.T) {
	res := IsRight(Left[string, string]("myVal"))
	if res != false {
		t.Error("IsRight should return false. Received:", res)
	}
}
func TestIsRight_Right(t *testing.T) {
	res := IsRight(Right[string]("myVal"))
	if res != true {
		t.Error("IsRight should return true. Received:", res)
	}
}

func TestExists_Left(t *testing.T) {
	res := Exists[string](isOdd)(Left[string, int]("myError"))
	if res != false {
		t.Error("Exists should return false. Received:", res)
	}
}

func TestExists_Right_TruePredicate(t *testing.T) {
	res := Exists[string](isOdd)(Right[string](1))
	if res != true {
		t.Error("Exists should mach the predicate and return true. Received:", res)
	}
}
func TestExists_Right_FalsePredicate(t *testing.T) {
	res := Exists[string](isOdd)(Right[string](2))
	if res != false {
		t.Error("Exists should not match the predicate and return true. Received:", res)
	}
}

func TestFlatten_RightRight(t *testing.T) {
	res := Flatten(Right[string](Right[string](1)))
	if res.isLeft != false {
		t.Error("Flatten should return the inner Right. Received:", res.isLeft)
	}
	if res.right != 1 {
		t.Error("Flatten should return the inner Right with the right value. Received:", res.right)
	}
}

func TestFlatten_LeftRight(t *testing.T) {
	leftVal := "myError"
	res := Flatten(Right[string](Left[string, int](leftVal)))
	if res.isLeft != true {
		t.Error("Flatten should return the inner Left. Received:", res.isLeft)
	}
	if res.left != leftVal {
		t.Error("Flatten should return the inner Left with the right value. Received:", res.right)
	}
}

func TestFlatten_Left(t *testing.T) {
	leftVal := "myError"
	res := Flatten(Left[string, Either[string, int]](leftVal))
	if res.isLeft != true {
		t.Error("Flatten should return a Left. Received:", res.isLeft)
	}
	if res.left != leftVal {
		t.Error("Flatten should return a Left with the left value. Received:", res.left)
	}
}

func TestFromError_error(t *testing.T) {
	res := FromError(strconv.Atoi("a"))
	if res.isLeft != true {
		t.Error("FromError should generate a Left. Received:", res.isLeft)
	}
}

func TestFromError_success(t *testing.T) {
	res := FromError(strconv.Atoi("1"))
	if res.isLeft != false {
		t.Error("FromError should generate a Right. Received:", res.isLeft)
	}
}

func TestFromErrorFn_error(t *testing.T) {
	res := FromErrorFn(func() (int, error) { return strconv.Atoi("a") })
	if res.isLeft != true {
		t.Error("FromErrorFn should generate a Left. Received:", res.isLeft)
	}
}

func TestFromErrorFn_success(t *testing.T) {
	res := FromErrorFn(func() (int, error) { return strconv.Atoi("1") })
	if res.isLeft != false {
		t.Error("FromErrorFn should generate a Right. Received:", res.isLeft)
	}
}

func TestFromOption_Some(t *testing.T) {
	rightVal := 1
	res := FromOption[string, int](func() string { return "onNone" })(opt.Some(rightVal))
	if res.isLeft != false {
		t.Error("FromOption should generate a Right. Received:", res.isLeft)
	}
	if res.right != rightVal {
		t.Error("FromOption should generate a Right with the Some value. Received:", res.right)
	}
}

func TestFromOption_None(t *testing.T) {
	leftVal := "myError"
	res := FromOption[string, int](func() string { return leftVal })(opt.None[int]())
	if res.isLeft != true {
		t.Error("FromOption should generate a Left. Received:", res.isLeft)
	}
	if res.left != leftVal {
		t.Error("FromOption should generate a Left with the onNone return value. Received:", res.left)
	}
}

func TestFromPredicate_Left(t *testing.T) {
	res := FromPredicate(isOdd, onLeft)(2)
	if res.isLeft != true {
		t.Error("FromPredicate should generate a Left. Received:", res.isLeft)
	}
	if res.left != "myOnLeftCallbackError" {
		t.Error("FromPredicate should generate a Left with the correct value. Received:", res.isLeft)
	}
}
func TestFromPredicate_Right(t *testing.T) {
	rightVal := 1
	res := FromPredicate(isOdd, onLeft)(rightVal)
	if res.isLeft != false {
		t.Error("FromPredicate should generate a Right. Received:", res.isLeft)
	}
	if res.right != rightVal {
		t.Error("FromPredicate should generate a Right with the correct value. Received:", res.isLeft)
	}
}

func TestGetOrElse_Left(t *testing.T) {
	res := GetOrElse(convertToIntAndAdd1)(Left[string, int]("7"))
	if res != 8 {
		t.Error("GetOrElse should return the result of the onLeft. Received:", res)
	}
}

func TestGetOrElse_Right(t *testing.T) {
	res := GetOrElse(convertToIntAndAdd1)(Right[string](3))
	if res != 3 {
		t.Error("GetOrElse should return the Right value. Received:", res)
	}
}

func TestMap_Left(t *testing.T) {
	leftVal := "myError"
	res := Map[string](isOdd)(Left[string, int](leftVal))
	if res.isLeft != true {
		t.Error("Map should not happen, Left should be returned. Received:", res.isLeft)
	}
	if res.left != leftVal {
		t.Error("Map should not happen, Left should be returned with the left value. Received:", res.left)
	}
}

func TestMap_Right(t *testing.T) {
	rightVal := 1
	res := Map[string](isOdd)(Right[string](rightVal))
	if res.isLeft != false {
		t.Error("Map should happen, Right should be returned. Received:", res.isLeft)
	}
	if res.right != true {
		t.Error("Map should happen, Right should be returned with the result of the onRight function. Received:", res.right)
	}
}

func TestMapLeft_Left(t *testing.T) {
	res := MapLeft[string, string](convertToIntAndAdd1)(Left[string, string]("1"))
	if res.isLeft != true {
		t.Error("MapLeft should happen, Left should be returned. Received:", res.isLeft)
	}
	if res.left != 2 {
		t.Error("MapLeft should happen, Left should be returned with the result of the callback. Received:", res.left)
	}
}

func TestMapLeft_Right(t *testing.T) {
	rightVal := "1"
	res := MapLeft[string, string](convertToIntAndAdd1)(Right[string](rightVal))
	if res.isLeft != false {
		t.Error("MapLeft should not happen, Right should be returned. Received:", res.isLeft)
	}
	if res.right != rightVal {
		t.Error("MapLeft should not happen, Right should be returned with the Right value. Received:", res.right)
	}
}

func TestMatch_Left(t *testing.T) {
	res := Match(convertToIntAndAdd1, convertToIntAndAdd2)(Left[string, string]("1"))
	if res != 2 {
		t.Error("Match should return the onLeft callback value. Received:", res)
	}
}
func TestMatch_Right(t *testing.T) {
	res := Match(convertToIntAndAdd1, convertToIntAndAdd2)(Right[string]("2"))
	if res != 4 {
		t.Error("Match should return the onRight callback value. Received:", res)
	}
}
