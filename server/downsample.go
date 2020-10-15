package server

import (
	"fmt"
	"math/rand"
	"sort"
)

// downsample picks k elements from a sequence of length n. It will always pick
// the last element; the remainder will be chosen uniformly from C([n-1], k-1).
// Panics if n < 0, k < 0, or k > n. Argument pick will be called once for each
// element picked. It may often be a closure that assigns dst[j] = src[i].
func downsample(n int, k int, pick func(srcIndex, dstIndex int)) {
	if n < 0 || k < 0 {
		panic(fmt.Sprintf("downsample got n=%v, k=%v; need non-negative", n, k))
	}
	if k > n {
		panic(fmt.Sprintf("downsample got n=%v, k=%v; need n >= k", n, k))
	}
	if n == 0 || k == 0 {
		return
	}
	pick(n-1, k-1)
	indices := make([]int, n-1)
	for i := range indices {
		indices[i] = i
	}
	rng := rand.New(rand.NewSource(0))
	rng.Shuffle(len(indices), func(i, j int) {
		indices[i], indices[j] = indices[j], indices[i]
	})
	chosen := indices[:k-1]
	sort.Ints(chosen)
	for dst, src := range chosen {
		pick(src, dst)
	}
}
