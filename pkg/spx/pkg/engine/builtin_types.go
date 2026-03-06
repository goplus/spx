package engine

type Node int64
type Object = int64
type Array = any

// The coefficient to multiply by when converting a floating-point number to int64
// equivalent to 4 decimal places
const Float2IntFactor = 10000
