package server

import (
	"sort"
	"strings"
	"testing"
)

func stringSlicesEqual(xs, ys []string) bool {
	if len(xs) != len(ys) {
		return false
	}
	for i, x := range xs {
		y := ys[i]
		if x != y {
			return false
		}
	}
	return true
}

func TestDownsampleEqualSize(t *testing.T) {
	src := strings.Fields("one two three four five six seven eight")
	dst := make([]string, len(src))
	downsample(len(src), len(dst), func(i, j int) {
		dst[j] = src[i]
	})
	if !stringSlicesEqual(src, dst) {
		t.Errorf("dst: got %v, want %v", dst, src)
	}
}

// TestDownsampleProperties performs a parameter sweep over `n` and `k`, and
// verifies that the downsampled output (a) always has the last input element
// in the last output position and (b) maintains the relative order of input
// elements in the output.
func TestDownsampleProperties(t *testing.T) {
	for n := 1; n < 100; n++ {
		for k := 1; k <= n; k++ {
			src := make([]int, n)
			dst := make([]int, k)
			for i := 0; i < n; i++ {
				src[i] = (i + 1) * (i + 1)
			}
			downsample(len(src), len(dst), func(i, j int) {
				dst[j] = src[i]
			})
			if !sort.IsSorted(sort.IntSlice(dst)) {
				t.Errorf("dst: got %v, want sorted (n=%v, k=%v)", src, n, k)
			}
			if dst[k-1] != n*n {
				t.Errorf("dst[k-1]: got %v, want %v (n=%v, k=%v)", dst[k-1], n*n, n, k)
			}
		}
	}
}
