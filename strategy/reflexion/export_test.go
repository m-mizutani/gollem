package reflexion

// Export private functions and types for testing

// NewMemory is exported for testing
var NewMemory = newMemory

// MemoryEntry is exported for testing with public fields
type MemoryEntry struct {
	TrialNum   int
	Reflection string
}

// Memory methods for testing
func (m *memory) Add(trialNum int, reflection string) {
	m.add(trialNum, reflection)
}

func (m *memory) GetAll() []MemoryEntry {
	entries := m.getAll()
	result := make([]MemoryEntry, len(entries))
	for i, e := range entries {
		result[i] = MemoryEntry{
			TrialNum:   e.trialNum,
			Reflection: e.reflection,
		}
	}
	return result
}

func (m *memory) Size() int {
	return m.size()
}
