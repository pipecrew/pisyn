package pisyn

// Stage groups jobs within a pipeline.
type Stage struct {
	Construct
	Name string
}

// NewStage creates a new stage in the given pipeline.
func NewStage(scope *Pipeline, name string) *Stage {
	s := &Stage{Name: name}
	s.Construct = newConstruct(&scope.Construct, name, s)
	return s
}

// Jobs returns all jobs in this stage.
func (s *Stage) Jobs() []*Job {
	return childrenOfType[Job](&s.Construct)
}
