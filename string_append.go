package otto

import (
	"unsafe"
)

const (
	// minAppendBuffer is the shortest string that concatenation tracks as one
	// that may be appended to again.
	minAppendBuffer = 256

	// appendSlots is how many strings, and how many buffers, are tracked.
	appendSlots = 4
)

// appendBuffer is spare capacity after the last string built in it. Strings
// built in buf are prefixes of it, and no string includes bytes past used, so
// those bytes can be written without changing any string.
type appendBuffer struct {
	buf  []byte
	used int
}

// appendState tracks recent concatenation results, so that a script
// appending to the same string over and over, as `s += x` in a loop does,
// costs O(n) for a string of n bytes, not O(n²).
//
// A result that is appended to is copied once into a buffer with room to
// grow, and later appends to the latest string in that buffer write into its
// spare capacity. A new buffer replaces the least recently appended to, so a
// string that is still growing keeps its buffer while each step builds and
// discards temporaries, as `s += "| " + row + " |"` does.
type appendState struct {
	strings    [appendSlots]string
	buffers    [appendSlots]appendBuffer
	bufferUse  [appendSlots]uint64
	uses       uint64
	nextString int
}

// useBuffer records that buffer i was appended to.
func (state *appendState) useBuffer(i int) {
	state.uses++
	state.bufferUse[i] = state.uses
}

// leastRecentBuffer returns the index of the buffer appended to least recently.
func (state *appendState) leastRecentBuffer() int {
	oldest := 0
	for i := 1; i < appendSlots; i++ {
		if state.bufferUse[i] < state.bufferUse[oldest] {
			oldest = i
		}
	}
	return oldest
}

// concatStrings returns left + right.
func (rt *runtime) concatStrings(left, right string) string {
	length := len(left) + len(right)
	rt.checkStringLength(length)
	if len(right) == 0 {
		return left
	}
	state := &rt.appendState
	if len(left) >= minAppendBuffer {
		data := unsafe.StringData(left) //nolint:gosec // Compared, never dereferenced.
		for i := range state.buffers {
			b := &state.buffers[i]
			if b.used != len(left) || len(b.buf) == 0 || &b.buf[0] != data {
				continue
			}
			if length > len(b.buf) {
				rt.growAppendBuffer(b, left, length)
			}
			copy(b.buf[b.used:], right)
			b.used = length
			state.useBuffer(i)
			return unsafe.String(&b.buf[0], length) //nolint:gosec // As strings.Builder does; see appendBuffer.
		}
		for i, str := range state.strings {
			if len(str) != len(left) || unsafe.StringData(str) != data { //nolint:gosec // Compared, never dereferenced.
				continue
			}
			state.strings[i] = ""
			slot := state.leastRecentBuffer()
			b := &state.buffers[slot]
			*b = appendBuffer{}
			rt.growAppendBuffer(b, left, length)
			copy(b.buf[len(left):], right)
			b.used = length
			state.useBuffer(slot)
			return unsafe.String(&b.buf[0], length) //nolint:gosec // As strings.Builder does; see appendBuffer.
		}
	}
	rt.allocate(int64(length))
	result := left + right
	if length >= minAppendBuffer {
		state.strings[state.nextString] = result
		state.nextString = (state.nextString + 1) % appendSlots
	}
	return result
}

// growAppendBuffer replaces b's buffer with one of at least twice length
// bytes, starting with prefix.
func (rt *runtime) growAppendBuffer(b *appendBuffer, prefix string, length int) {
	size := 2 * length
	if rt.stringLimit > 0 {
		size = min(size, max(rt.stringLimit, length))
	}
	rt.allocate(int64(size))
	buf := make([]byte, size)
	copy(buf, prefix)
	b.buf = buf
	b.used = len(prefix)
}
