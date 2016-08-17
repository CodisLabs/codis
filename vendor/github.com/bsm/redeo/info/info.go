package info

// Main info registry.
// Please note: in order to minimise performance impact info registries
// are not using locks are therefore not thread-safe. Please make sure
// you register all metrics and values before you start the server.
type Registry struct{ sections []*Section }

// New creates a new Registry
func New() *Registry {
	return &Registry{make([]*Section, 0)}
}

// Section returns a section, or appends a new one
// when the given name cannot be found
func (r *Registry) Section(name string) *Section {
	for _, s := range r.sections {
		if s.name == name {
			return s
		}
	}
	section := &Section{name: name, kvs: make([]kv, 0)}
	r.sections = append(r.sections, section)
	return section
}

// Clear removes all sections from the registry
func (r *Registry) Clear() {
	r.sections = r.sections[:0]
}

// String generates an info string output
func (r *Registry) String() string {
	result := ""
	for _, section := range r.sections {
		if len(section.kvs) > 0 {
			result += "# " + section.name + "\n" + section.String() + "\n"
		}
	}
	if len(result) > 1 {
		result = result[:len(result)-1]
	}
	return result
}

// An info section contains multiple values
type Section struct {
	name string
	kvs  []kv
}

// Register registers a value under a name
func (s *Section) Register(name string, value Value) {
	s.kvs = append(s.kvs, kv{name, value})
}

// Clear removes all values from a section
func (s *Section) Clear() {
	s.kvs = s.kvs[:0]
}

// String generates a section string output
func (s *Section) String() string {
	result := ""
	for _, kv := range s.kvs {
		result += kv.name + ":" + kv.value.String() + "\n"
	}
	return result
}

type kv struct {
	name  string
	value Value
}
