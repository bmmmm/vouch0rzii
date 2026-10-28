// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import "math"

// hungarian solves the assignment problem with n rows and m ≥ n columns:
// cost[i][j] is the cost of giving row i column j. It returns, per row, the
// chosen column with minimal total cost. Deterministic for equal inputs.
// O(n²·m).
func hungarian(cost [][]int) []int {
	n := len(cost)
	if n == 0 {
		return nil
	}
	m := len(cost[0])
	if m < n {
		panic("hungarian: more rows than columns")
	}
	const inf = math.MaxInt / 4
	u := make([]int, n+1)
	v := make([]int, m+1)
	p := make([]int, m+1)   // p[j] = row matched to column j (1-based), 0 = free
	way := make([]int, m+1) // way[j] = previous column on the augmenting path
	for i := 1; i <= n; i++ {
		p[0] = i
		j0 := 0
		minv := make([]int, m+1)
		used := make([]bool, m+1)
		for j := range minv {
			minv[j] = inf
		}
		for {
			used[j0] = true
			i0, delta, j1 := p[j0], inf, 0
			for j := 1; j <= m; j++ {
				if used[j] {
					continue
				}
				cur := cost[i0-1][j-1] - u[i0] - v[j]
				if cur < minv[j] {
					minv[j] = cur
					way[j] = j0
				}
				if minv[j] < delta {
					delta = minv[j]
					j1 = j
				}
			}
			for j := 0; j <= m; j++ {
				if used[j] {
					u[p[j]] += delta
					v[j] -= delta
				} else {
					minv[j] -= delta
				}
			}
			j0 = j1
			if p[j0] == 0 {
				break
			}
		}
		for {
			j1 := way[j0]
			p[j0] = p[j1]
			j0 = j1
			if j0 == 0 {
				break
			}
		}
	}
	out := make([]int, n)
	for j := 1; j <= m; j++ {
		if p[j] != 0 {
			out[p[j]-1] = j - 1
		}
	}
	return out
}
