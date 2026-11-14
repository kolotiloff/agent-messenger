package platform

// Member is one process of a run's group, by name (T-172). Names describe a
// run that has not ended; they are never evidence that it ended.
type Member struct {
	PID  int
	Name string
}
