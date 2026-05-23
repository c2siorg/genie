package prompt

import "math"

// Single import surface for the small math functions bandit.go needs;
// keeps the bandit file readable without sprinkling math.* everywhere.
var (
	mathPow  = math.Pow
	mathSqrt = math.Sqrt
	mathLog  = math.Log
)
