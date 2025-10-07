package sample

// deadVar is defined but never used.
var deadVar = 42

// deadConst is defined but never used.
const deadConst = "unused"

// deadType is defined but never used.
type deadType struct {
	X int
}

// deadFunc is defined but never used.
func deadFunc() string {
	return "not used"
}
