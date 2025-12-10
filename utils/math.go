package utils

type Integer interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}

func Abs[T Integer](x T) T {
	if x < 0 {
		return -x
	}
	return x
}

func Divmod[T Integer](x, y T) (T, T) {
	return x / y, x % y
}

func Gcd[T Integer](a, b T) T {
	for b > 0 {
		a, b = b, a%b
	}
	return a
}

func Lcm[T Integer](a, b T) T {
	return a * b / Gcd(a, b)
}

func QucikPow[T, V Integer](x T, n V, mod T) T {
	res := T(1)
	for ; n > 0; n >>= 1 {
		if n&1 == 1 {
			res = res * x % mod
		}
		x = x * x % mod
	}
	return res
}
