package reporter

func NewReporter() Reporter {
	return newEmailReport()
}

type Reporter interface {
	Send(string) error
}
