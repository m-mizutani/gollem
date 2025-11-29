package reflexion

// memory manages episodic memory (reflections from past trials).
// It maintains a bounded FIFO buffer of reflections.
type memory struct {
	reflections []memoryEntry
	maxSize     int
}

// memoryEntry represents a single reflection entry in episodic memory.
type memoryEntry struct {
	trialNum   int
	reflection string
}

// newMemory creates a new memory instance with the specified maximum size.
func newMemory(maxSize int) *memory {
	return &memory{
		reflections: make([]memoryEntry, 0, maxSize),
		maxSize:     maxSize,
	}
}

// add adds a new reflection to memory.
// If the memory is full, the oldest entry is removed (FIFO).
func (m *memory) add(trialNum int, reflection string) {
	entry := memoryEntry{
		trialNum:   trialNum,
		reflection: reflection,
	}

	m.reflections = append(m.reflections, entry)

	// FIFO: remove oldest if exceeds maxSize
	if len(m.reflections) > m.maxSize {
		m.reflections = m.reflections[1:]
	}
}

// getAll returns all memory entries.
func (m *memory) getAll() []memoryEntry {
	return m.reflections
}

// size returns the current number of entries in memory.
func (m *memory) size() int {
	return len(m.reflections)
}
