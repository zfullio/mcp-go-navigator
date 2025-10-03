package sample

func Simple() int {
	return 1
}

func WithIf(x int) int {
	if x > 0 {
		return x
	}
	return -x
}

func WithLoopAndSwitch(n int) int {
	sum := 0
	for i := 0; i < n; i++ {
		switch {
		case i%2 == 0:
			sum += i
		default:
			sum -= i
		}
	}
	return sum
}
